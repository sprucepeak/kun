// Package gen provides the "kun gen db" subcommand.
// It connects to a database (or parses a SQL file) and generates
// GORM repository files for the specified tables.
package gen

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/spf13/cobra"
	"github.com/sprucepeak/kun/internal/gen/kernel"
	"github.com/sprucepeak/kun/pkg/output"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Database type
type DBType string

const (
	DefaultOutPath = "./internal/repository/db"
	VersionText    = "数据库生成GORM Repository文件"

	// dbMySQL Gorm Drivers mysql || postgres || clickhouse || sqlite
	dbMySQL      DBType = "mysql"
	dbPostgres   DBType = "postgres"
	dbClickHouse DBType = "clickhouse"
	dbSQLite     DBType = "sqlite"
)

// CmdParams is command line parameters
type CmdParams struct {
	DSN     string   // user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local
	Tables  []string // 输入所需的数据表或将其留空,留空数据库中所有的数据表
	OutPath string   // 指定输出目录
	Prefix  string   // 表前缀,不为空则model不包含前缀
	DBType  string   // 数据库类型
}

// driverRegistry 驱动映射表（均采用纯 Go 驱动实现，无 CGO 依赖，支持纯静态构建与全平台交叉编译）
var driverRegistry = map[DBType]func(string) gorm.Dialector{
	dbMySQL:      func(dsn string) gorm.Dialector { return mysql.Open(dsn) },
	dbPostgres:   func(dsn string) gorm.Dialector { return postgres.Open(dsn) },
	dbSQLite:     func(dsn string) gorm.Dialector { return sqlite.Open(dsn) },
	dbClickHouse: func(dsn string) gorm.Dialector { return clickhouse.Open(dsn) },
}

// connectDB 连接数据库 选择用于连接到数据库的数据库类型
func connectDB(t DBType, dsn string) (*gorm.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("dsn cannot be empty")
	}
	opener, ok := driverRegistry[t]
	if !ok {
		return nil, fmt.Errorf("driver %q is not supported", t)
	}
	return gorm.Open(opener(dsn))
}

func detectDBType(dsn string) DBType {
	dsnLower := strings.ToLower(dsn)
	if strings.Contains(dsnLower, "host=") || strings.Contains(dsnLower, "sslmode=") || strings.HasPrefix(dsnLower, "postgres://") || strings.HasPrefix(dsnLower, "postgresql://") {
		return dbPostgres
	}
	if strings.Contains(dsnLower, "clickhouse://") || (strings.Contains(dsnLower, "tcp://") && strings.Contains(dsnLower, "9000")) {
		return dbClickHouse
	}
	if strings.HasSuffix(dsnLower, ".db") || strings.HasSuffix(dsnLower, ".sqlite") || strings.HasSuffix(dsnLower, ".sqlite3") || strings.Contains(dsnLower, "file:") {
		return dbSQLite
	}
	return dbMySQL
}

func genDBRepo(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// 如果参数是 .sql 文件，则解析 SQL 文件生成 repo
	if strings.HasSuffix(args[0], ".sql") {
		return genDBRepoFromSQL(args, dryRun)
	}

	cmdConf := &CmdParams{
		DSN:     args[0],
		OutPath: DefaultOutPath,
	}
	cmdConf.DBType = string(detectDBType(cmdConf.DSN))
	if len(args) > 1 && args[1] != "" {
		if args[1] == "*" {
			cmdConf.Tables = []string{}
		} else {
			cmdConf.Tables = strings.Split(args[1], ",")
		}
	}

	gormDb, err := connectDB(DBType(cmdConf.DBType), cmdConf.DSN)
	if err != nil {
		// 连接错误信息可能回显 DSN 片段(含明文密码),脱敏后再返回。
		return fmt.Errorf("connect db server fail: %w", maskDSN(err))
	}
	if gormDb == nil {
		return fmt.Errorf("gorm db is nil")
	}
	// 生成完成后显式关闭底层 *sql.DB,避免 CLI 进程依赖退出才释放连接。
	if sqlDB, err := gormDb.DB(); err == nil {
		defer sqlDB.Close()
	}
	// 自定义命名策略
	gormDb.Config.NamingStrategy = schema.NamingStrategy{
		TablePrefix:   cmdConf.Prefix,
		SingularTable: true,
		NameReplacer:  nil,
	}
	conf := defaultSQLConfig()
	conf.DbConn = gormDb
	g := kernel.NewGenerator(conf)

	var tablesList []string
	if len(cmdConf.Tables) == 0 {
		// Execute tasks for all tables in the database
		tablesList, err = getDBTables(gormDb, DBType(cmdConf.DBType))
		if err != nil {
			return fmt.Errorf("GORM migrator get all tables fail: %w", maskDSN(err))
		}
		if len(tablesList) == 0 {
			output.Warn("no tables found in database")
			return nil
		}
	} else {
		tablesList = cmdConf.Tables
	}

	if dryRun {
		output.Success("[dry-run] will generate db repository for tables: %v to %s", tablesList, cmdConf.OutPath)
		return nil
	}

	// 汇总各表的失败原因:单表失败不中断其它表的生成,但最终必须以错误返回,
	// 否则 CLI 打印红色错误却以退出码 0 结束,CI 无法拦截。
	var genErrs []error
	for _, tableName := range tablesList {
		if err := g.GenerateRepo(tableName); err != nil {
			output.Error("%s", maskDSN(err))
			genErrs = append(genErrs, maskDSN(err))
		}
	}

	if err := g.Execute(); err != nil {
		genErrs = append(genErrs, maskDSN(err))
	}
	if len(genErrs) > 0 {
		return errors.Join(genErrs...)
	}
	return nil
}

// getDBTables 获取数据库中所有表名，针对 ClickHouse 等缺少 information_schema 的数据库提供原生系统表及 SHOW TABLES 兜底支持
func getDBTables(gormDb *gorm.DB, dbType DBType) ([]string, error) {
	if dbType == dbClickHouse {
		var tablesList []string
		// 1. 优先从 ClickHouse 原生系统表 system.tables 查询当前数据库下的所有常规非临时表
		if err := gormDb.Raw("SELECT name FROM system.tables WHERE database = currentDatabase() AND is_temporary = 0").Scan(&tablesList).Error; err == nil {
			return tablesList, nil
		}
		// 2. 降级使用 ClickHouse 原生 SHOW TABLES 语法
		if err := gormDb.Raw("SHOW TABLES").Scan(&tablesList).Error; err == nil {
			return tablesList, nil
		}
	}

	// 默认使用 GORM 官方 Migrator.GetTables()
	tablesList, err := gormDb.Migrator().GetTables()
	if err != nil {
		// 3. 容错降级：如果 Migrator 报错（如 ClickHouse 或精简库提示 information_schema doesn't exist）
		if dbType == dbClickHouse || strings.Contains(strings.ToLower(err.Error()), "information_schema") {
			var fallbackTables []string
			if fErr := gormDb.Raw("SHOW TABLES").Scan(&fallbackTables).Error; fErr == nil {
				return fallbackTables, nil
			}
		}
		return nil, err
	}
	return tablesList, nil
}

// maskDSN 将错误信息中可能出现的 DSN 密码替换为 ***,避免明文密码进终端/CI 日志。
//
// B3: 此处故意使用 fmt.Errorf("%s", msg) 而非 %w，因为原始错误中包含明文密码片段。
// 使用 %w 会保留原始错误对象，导致调用方通过 errors.As/Unwrap 拿到含密码的错误文本。
// 权衡：安全性优先于错误链的可追溯性；调用方若需类型判断，应在 maskDSN 之前处理。
func maskDSN(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	// 1. MySQL DSN 形如 user:password@tcp(host)/dbname —— 把第一个 ':' 与 '@tcp' 之间的内容掩码。
	if at := strings.Index(msg, "@tcp("); at > 0 {
		if colon := strings.Index(msg[:at], ":"); colon >= 0 && colon < at {
			msg = msg[:colon+1] + "***" + msg[at:]
		}
	}
	// 2. URL 风格 DSN (clickhouse://user:password@host, postgres://user:password@host 等)
	if scheme := strings.Index(msg, "://"); scheme > 0 {
		rest := msg[scheme+3:]
		if at := strings.Index(rest, "@"); at > 0 {
			userPass := rest[:at]
			if colon := strings.Index(userPass, ":"); colon >= 0 {
				msg = msg[:scheme+3+colon+1] + "***" + msg[scheme+3+at:]
			}
		}
	}
	return fmt.Errorf("%s", msg)
}

// defaultSQLConfig 构建默认 SQLConfig（DB 和 SQL 文件路径共用）
func defaultSQLConfig() kernel.SQLConfig {
	outPath, err := filepath.Abs(DefaultOutPath)
	if err != nil {
		output.Error("outPath is invalid: %s, using default", err)
		outPath = DefaultOutPath
	}
	return kernel.SQLConfig{
		OutPath:           outPath,
		PackageName:       "db",
		FieldCoverable:    false, // 当字段具有默认值时生成指针，以解决无法分配零值的问题
		FieldNullable:     true,  // 当字段可为空时生成指针。注意：此默认值会让所有可空列生成 *T 指针类型，若希望生成值类型请改为 false
		FieldWithIndexTag: true,  // 生成字段包含 索引 标记
		FieldWithTypeTag:  true,  // 生成字段包含 列类型 标记
		FieldSignable:     false, // 检测整数字段的无符号类型，调整生成的数据类型
	}
}

// genDBRepoFromSQL 从 .sql 文件解析 CREATE TABLE 语句并生成 repo
func genDBRepoFromSQL(args []string, dryRun bool) error {
	conf := defaultSQLConfig()

	metas, err := kernel.ParseSQLFile(args[0], &conf)
	if err != nil {
		return fmt.Errorf("parse sql file fail: %w", err)
	}
	if len(metas) == 0 {
		output.Warn("no CREATE TABLE found in file: %s", args[0])
		return nil
	}

	var tableFilter []string
	if len(args) > 1 && args[1] != "" && args[1] != "*" {
		tableFilter = strings.Split(args[1], ",")
	}

	g := kernel.NewGenerator(conf)

	for _, meta := range metas {
		if meta == nil {
			continue
		}
		if len(tableFilter) > 0 {
			found := false
			for _, t := range tableFilter {
				if strings.EqualFold(meta.TableName, t) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		g.AddRepoMeta(meta)
	}

	if dryRun {
		output.Success("[dry-run] will generate db repository from %s for %d tables to %s", args[0], len(metas), conf.OutPath)
		return nil
	}

	if err := g.Execute(); err != nil {
		return err
	}
	return nil
}
