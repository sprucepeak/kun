/**
 * @Desc: 解析 SQL 文件中的 CREATE TABLE 语句，生成 StructMeta
 */

package kernel

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/sprucepeak/kun/pkg/output"
	"gorm.io/gorm/schema"
)

// 预编译正则表达式
var (
	reCreateTable = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + "`?" + `(\w+)` + "`?" + `\s*\(`)
	// rePrimaryKey 捕获括号内的完整列清单,而不是单个列。
	// 必须如此才能识别复合主键 PRIMARY KEY (`a`,`b`) 与前缀主键 PRIMARY KEY (`name`(10));
	// 只匹配单列的写法会让这两种主键静默丢失,生成出缺少 Find/Update/Delete 的 repo。
	rePrimaryKey = regexp.MustCompile(`(?i)PRIMARY\s+KEY\s*\(([^)]*(?:\([^)]*\)[^)]*)*)\)`)
	// rePKColumn 从列清单里逐个取列名,忽略 (10) 这类前缀长度与 ASC/DESC 修饰。
	rePKColumn = regexp.MustCompile("`?(\\w+)`?(?:\\s*\\(\\s*\\d+\\s*\\))?")
	// P4: 兼容索引名有/无反引号两种写法（标准 MySQL 有反引号，部分方言无）
	reUniqueKey   = regexp.MustCompile(`(?i)UNIQUE\s+KEY\s+` + "`?" + `(\w+)` + "`?" + `\s*\(([^)]+)\)`)
	reComment     = regexp.MustCompile(`(?i)COMMENT\s+'((?:[^']|''|\\.)*)'`)
	reDefault     = regexp.MustCompile(`(?i)DEFAULT\s+('[^']*'|[\w.]+|NULL)`)
	reNoiseClause = regexp.MustCompile(`(?i)\b(CHARACTER\s+SET\s+\w+|COLLATE\s+\w+)\b`)
)

// columnDef 表示从 CREATE TABLE 语句中解析出的列定义
type columnDef struct {
	Name        string // 原始列名
	SQLType     string // 完整 SQL 类型，如 "decimal(10,2) unsigned"
	RawType     string // 基础类型，如 "decimal"
	Constraints string // 剩余约束
	IsUnsigned  bool   // 是否有 UNSIGNED 修饰
}

// uniqueIndexInfo 表示 UNIQUE KEY 索引信息
type uniqueIndexInfo struct {
	IndexName string
	Priority  int // 索引中的列顺序，从 1 开始
}

// ParseSQLFile 解析 .sql 文件，提取所有 CREATE TABLE 语句并构建 StructMeta
// conf 用于控制代码生成行为（FieldNullable、FieldCoverable、FieldSignable、FieldWithIndexTag、FieldWithTypeTag）
func ParseSQLFile(filePath string, conf *SQLConfig) ([]*StructMeta, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read sql file fail: %w", err)
	}

	// 处理 BOM 和 UTF-16 编码（Windows 系统可能将文件保存为 UTF-16 LE）
	content = convertToUTF8(content)

	// 剥离 SQL 注释。mysqldump 的输出充满 `--`、`/* ... */` 与 `/*!40101 ... */`,
	// 不剥离会让注释里的建表语句片段/关键字污染解析结果(解析出错表、错主键)。
	sql := stripSQLComments(string(content))

	var metas []*StructMeta
	remaining := sql
	for {
		loc := reCreateTable.FindStringSubmatchIndex(remaining)
		if loc == nil {
			break
		}
		tableName := remaining[loc[2]:loc[3]]

		// loc[1]-1 是 '(' 的位置，找到匹配的 ')'
		parenStart := loc[1] - 1
		parenEnd := findMatchingParen(remaining, parenStart)
		if parenEnd < 0 {
			output.Warn("skip table <%s>: cannot find matching ')'", tableName)
			remaining = remaining[loc[1]:]
			continue
		}

		body := remaining[parenStart+1 : parenEnd]
		meta := parseTableBody(tableName, body, conf)
		if meta != nil {
			metas = append(metas, meta)
		}

		remaining = remaining[parenEnd+1:]
	}

	return metas, nil
}

// stripSQLComments 移除 SQL 注释,保留字符串字面量与反引号标识符里的内容。
//
// 需要处理三种注释:
//   - `-- ...` 到行尾
//   - `# ...` 到行尾(MySQL 扩展)
//   - `/* ... */` 块注释(含 mysqldump 的 /*!40101 ... */ 条件注释)
//
// 关键点:必须跟踪引号状态。列注释里出现 `--`、`#` 或 `/*`(中文注释里很常见)时,
// 若不判断引号就剥离,会把 COMMENT '...' 截断成不配对的引号,导致后续解析全部错位。
func stripSQLComments(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	var (
		inSingle bool // '...'
		inDouble bool // "..."
		inTick   bool // `...`
	)
	for i := 0; i < len(s); i++ {
		c := s[i]

		if inSingle {
			b.WriteByte(c)
			switch c {
			case '\\':
				// 反斜杠转义:连带下一个字节一起写出,避免 \' 被误判为结束引号
				if i+1 < len(s) {
					i++
					b.WriteByte(s[i])
				}
			case '\'':
				// '' 是单引号自身的转义写法
				if i+1 < len(s) && s[i+1] == '\'' {
					i++
					b.WriteByte(s[i])
				} else {
					inSingle = false
				}
			}
			continue
		}
		if inDouble {
			b.WriteByte(c)
			switch c {
			case '\\':
				if i+1 < len(s) {
					i++
					b.WriteByte(s[i])
				}
			case '"':
				if i+1 < len(s) && s[i+1] == '"' {
					i++
					b.WriteByte(s[i])
				} else {
					inDouble = false
				}
			}
			continue
		}
		if inTick {
			b.WriteByte(c)
			if c == '`' {
				inTick = false
			}
			continue
		}

		switch {
		case c == '\'':
			inSingle = true
			b.WriteByte(c)
		case c == '"':
			inDouble = true
			b.WriteByte(c)
		case c == '`':
			inTick = true
			b.WriteByte(c)
		case c == '#', c == '-' && i+1 < len(s) && s[i+1] == '-':
			// 行注释:跳到行尾,保留换行以维持行结构
			for i < len(s) && s[i] != '\n' {
				i++
			}
			if i < len(s) {
				b.WriteByte('\n')
			}
		case c == '/' && i+1 < len(s) && s[i+1] == '*':
			// 块注释:跳到 */,用一个空格替代,避免把两侧 token 粘连成一个
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			i++ // 此时 i 指向 '*',跳过它;for 的 i++ 再跳过 '/'
			b.WriteByte(' ')
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// stripQuotedLiterals 把单引号字符串字面量替换为空格,用于安全地扫描约束关键字。
// 保留字面量以外的所有内容(含反引号标识符),仅消除引号内文本的干扰。
func stripQuotedLiterals(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inSingle := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inSingle {
			switch c {
			case '\\':
				i++ // 跳过被转义的字符
			case '\'':
				if i+1 < len(s) && s[i+1] == '\'' {
					i++ // '' 转义,仍在字面量内
				} else {
					inSingle = false
					b.WriteByte(' ')
				}
			}
			continue
		}
		if c == '\'' {
			inSingle = true
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// findMatchingParen 从 start 位置开始找到匹配的右括号，跳过字符串字面量
func findMatchingParen(s string, start int) int {
	depth := 0
	inString := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			if ch == '\\' && i+1 < len(s) {
				i++ // 跳过反斜杠转义的字符
				continue
			}
			if ch == '\'' {
				// 检查转义的单引号 ''
				if i+1 < len(s) && s[i+1] == '\'' {
					i++ // 跳过转义的单引号
				} else {
					inString = false
				}
			}
			continue
		}
		if ch == '\'' {
			inString = true
			continue
		}
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// parseTableBody 解析 CREATE TABLE 的列定义部分
func parseTableBody(tableName, body string, conf *SQLConfig) *StructMeta {
	structName := schema.NamingStrategy{}.SchemaName(tableName)
	fileName := toLowerCamel(structName)

	columns := splitByComma(body)

	// 第一遍：解析 UNIQUE KEY 索引信息
	uniqueIndexMap := parseUniqueIndexes(body)

	var fields []*Field
	primaryKeyType := "int64"
	// compositePK 记录是否为复合主键。生成的 repo(Find/Update/Delete by id)
	// 只支持单列主键,复合主键必须显式告警,否则用户拿到的是静默残缺的代码。
	compositePK := false

	for _, col := range columns {
		col = strings.TrimSpace(col)
		if col == "" {
			continue
		}

		// 跳过约束/索引定义行
		if isConstraintLine(col) {
			// 提取 PRIMARY KEY 中的列名来标记主键(可能是复合主键)
			pkCols := extractPKColumns(col)
			if len(pkCols) > 0 {
				compositePK = len(pkCols) > 1
				for _, pkCol := range pkCols {
					for _, f := range fields {
						if strings.EqualFold(f.ColumnName, pkCol) {
							f.IsPrimaryKey = true
							primaryKeyType = f.Type
							if !strings.Contains(f.GORMTag, "primaryKey") {
								f.GORMTag += ";primaryKey"
							}
						}
					}
				}
			}
			continue
		}

		cd := parseColumn(col)
		if cd == nil {
			continue
		}

		field := buildField(cd, uniqueIndexMap[cd.Name], conf)
		if field.IsPrimaryKey {
			primaryKeyType = field.Type
		}
		fields = append(fields, field)
	}

	// E8: 使用公共 findPrimaryKey 函数，消除与 sqlGenerator.go 的重复逻辑。
	var hasPrimaryKey bool
	var primaryKeyName, primaryKeyColumn string
	var primaryKeyAutoIncrement bool
	primaryKeyName, primaryKeyColumn, primaryKeyType, primaryKeyAutoIncrement, hasPrimaryKey = findPrimaryKey(fields)

	if structName == "" {
		return nil
	}

	seenFields := make(map[string]string)
	for _, f := range fields {
		if prevCol, exists := seenFields[f.Name]; exists {
			output.Warn("table %q 中列 %q 与 %q 映射到重复的 Go 字段名 %q", tableName, prevCol, f.ColumnName, f.Name)
		} else {
			seenFields[f.Name] = f.ColumnName
		}
	}

	// 复合主键与"完全没有主键"都会让生成的 repo 残缺(Find/Update/Delete by id 无法工作),
	// 必须显式告警 —— 静默生成半成品代码远比报错更难排查。
	if compositePK {
		output.Warn("table %q 使用复合主键,生成的 repo 仅支持单列主键(Find/Update/Delete by id),请手工调整", tableName)
	}
	if !hasPrimaryKey {
		output.Warn("table %q 未识别到主键,生成的 repo 将缺少 Find/Update/Delete 等按主键操作的方法", tableName)
	}

	return &StructMeta{
		FileName:                fileName,
		InterfaceName:           fileName,
		StructName:              structName,
		TableName:               tableName,
		PackageName:             "db",
		PrimaryKeyType:          primaryKeyType,
		Fields:                  fields,
		HasPrimaryKey:           hasPrimaryKey,
		PrimaryKeyName:          primaryKeyName,
		PrimaryKeyColumn:        primaryKeyColumn,
		PrimaryKeyAutoIncrement: primaryKeyAutoIncrement,
		HasSoftDelete:           hasSoftDeleteField(fields),
	}
}

// splitByComma 按逗号切分，正确处理括号嵌套和字符串字面量
func splitByComma(s string) []string {
	var parts []string
	depth := 0
	inString := false
	start := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			if ch == '\\' && i+1 < len(s) {
				i++ // 跳过反斜杠转义的字符
				continue
			}
			if ch == '\'' {
				// 检查转义的单引号 ''
				if i+1 < len(s) && s[i+1] == '\'' {
					i++ // 跳过转义的单引号
				} else {
					inString = false
				}
			}
			continue
		}
		if ch == '\'' {
			inString = true
			continue
		}
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

// isConstraintLine 判断是否是约束/索引行
func isConstraintLine(line string) bool {
	upper := strings.ToUpper(strings.TrimSpace(line))
	for _, prefix := range [...]string{"PRIMARY KEY", "KEY ", "INDEX ", "UNIQUE KEY", "UNIQUE INDEX", "CONSTRAINT", "FULLTEXT", "SPATIAL"} {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	// B5: 兼容 KEY` 无空格形式（部分 SQL dump 省略 KEY 与反引号间的空格）
	if strings.HasPrefix(upper, "KEY`") {
		return true
	}
	return false
}

// extractPKColumns 从 PRIMARY KEY (...) 中提取全部列名。
// 支持单列 (`id`)、复合主键 (`a`,`b`)、前缀主键 (`name`(10)) 以及尾部的 USING BTREE。
func extractPKColumns(line string) []string {
	m := rePrimaryKey.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	var cols []string
	for _, raw := range strings.Split(m[1], ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if cm := rePKColumn.FindStringSubmatch(raw); cm != nil && cm[1] != "" {
			cols = append(cols, cm[1])
		}
	}
	return cols
}

// parseUniqueIndexes 从 CREATE TABLE body 中解析 UNIQUE KEY 约束
func parseUniqueIndexes(body string) map[string][]uniqueIndexInfo {
	result := make(map[string][]uniqueIndexInfo)
	for _, m := range reUniqueKey.FindAllStringSubmatch(body, -1) {
		idxName := m[1]
		cols := strings.Split(m[2], ",")
		for i, col := range cols {
			col = strings.Trim(strings.TrimSpace(col), "`")
			result[col] = append(result[col], uniqueIndexInfo{
				IndexName: idxName,
				Priority:  i + 1,
			})
		}
	}
	return result
}

// parseColumn 手写解析器：列名 + 类型（含嵌套括号） + UNSIGNED? + 剩余约束
func parseColumn(line string) *columnDef {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	// 1. 提取列名（可能被反引号包裹）
	colName, rest := extractQuotedName(line)
	if colName == "" {
		return nil
	}

	// 2. 提取类型：单词 + 可选括号参数（支持平衡括号）
	rawType, typeParams, rest := extractTypeWithParens(rest)
	if rawType == "" {
		return nil
	}

	fullType := strings.ToLower(rawType + typeParams)

	// 3. 检测 UNSIGNED / SIGNED
	isUnsigned := false
	rest = strings.TrimSpace(rest)
	if len(rest) >= 8 && strings.EqualFold(rest[:8], "UNSIGNED") {
		isUnsigned = true
		rest = strings.TrimSpace(rest[8:])
	} else if len(rest) >= 6 && strings.EqualFold(rest[:6], "SIGNED") {
		rest = strings.TrimSpace(rest[6:])
	}

	// 4. 移除噪声子句: CHARACTER SET xxx, COLLATE xxx
	rest = reNoiseClause.ReplaceAllString(rest, "")
	rest = compactSpaces(rest)

	// 5. 构建完整 SQL 类型（含 unsigned）
	sqlType := fullType
	if isUnsigned {
		sqlType += " unsigned"
	}

	return &columnDef{
		Name:        colName,
		SQLType:     strings.ToLower(sqlType),
		RawType:     strings.ToLower(rawType),
		Constraints: rest,
		IsUnsigned:  isUnsigned,
	}
}

// extractQuotedName 提取反引号包裹或裸单词的列名，返回 (name, remaining)
func extractQuotedName(s string) (string, string) {
	if strings.HasPrefix(s, "`") {
		end := strings.Index(s[1:], "`")
		if end < 0 {
			return "", s
		}
		return s[1 : end+1], strings.TrimSpace(s[end+2:])
	}
	// 裸单词: 支持空格与 Tab 分隔
	fields := strings.Fields(s)
	if len(fields) < 2 {
		return "", s
	}
	name := fields[0]
	return name, strings.TrimSpace(s[len(name):])
}

// extractTypeWithParens 提取类型单词 + 平衡括号参数，返回 (rawType, params, remaining)
func extractTypeWithParens(s string) (string, string, string) {
	s = strings.TrimSpace(s)
	spaceIdx := strings.IndexFunc(s, unicode.IsSpace)
	parenIdx := strings.Index(s, "(")

	// 没有空格和括号 → 整个字符串就是类型
	if spaceIdx < 0 && parenIdx < 0 {
		return s, "", ""
	}

	// 类型名后紧跟括号: type(params)...
	if parenIdx >= 0 && (spaceIdx < 0 || parenIdx < spaceIdx) {
		rawType := s[:parenIdx]
		rest := s[parenIdx:]
		closeIdx := findMatchingParen(rest, 0)
		if closeIdx < 0 {
			return s, "", ""
		}
		params := normalizeTypeParams(rest[:closeIdx+1])
		remaining := strings.TrimSpace(rest[closeIdx+1:])
		return rawType, params, remaining
	}

	if spaceIdx > 0 {
		return s[:spaceIdx], "", strings.TrimSpace(s[spaceIdx:])
	}

	return s, "", ""
}

// normalizeTypeParams 去掉类型参数中逗号前后的空格: "(10, 2)" → "(10,2)"
func normalizeTypeParams(params string) string {
	if len(params) < 2 {
		return params
	}
	return "(" + strings.ReplaceAll(params[1:len(params)-1], " ", "") + ")"
}

// buildField 根据解析的列定义构建 kernel.Field
func buildField(cd *columnDef, uniqueIdxs []uniqueIndexInfo, conf *SQLConfig) *Field {
	// nil 安全兜底
	if conf == nil {
		conf = &SQLConfig{
			FieldNullable:     true,
			FieldCoverable:    false,
			FieldSignable:     false,
			FieldWithIndexTag: true,
			FieldWithTypeTag:  true,
		}
	}

	// 列名转 Go 字段名: snake_case → PascalCase, ID → Id
	fieldName := schema.NamingStrategy{}.SchemaName(cd.Name)
	fieldName = strings.ReplaceAll(fieldName, "ID", "Id")

	goType := GetSQLGoType(cd.RawType, cd.SQLType)

	// FieldSignable: 检测无符号整数，调整数据类型
	if conf.FieldSignable && cd.IsUnsigned && strings.HasPrefix(goType, "int") {
		goType = "u" + goType
	}

	isPK := false
	isAutoIncr := false
	isNullable := true // 默认 nullable，遇到 NOT NULL 才置 false
	defaultVal := ""
	comment := ""

	// 提取 COMMENT 并消毒换行符
	if cre := reComment.FindStringSubmatch(cd.Constraints); cre != nil {
		comment = unescapeSQLString(cre[1])
		comment = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(comment)
	}

	// 提取 DEFAULT 前必须先剔除 COMMENT 避免 COMMENT 内部的 DEFAULT 关键字污染默认值
	constraintsWithoutComment := reComment.ReplaceAllString(cd.Constraints, "")
	if dre := reDefault.FindStringSubmatch(constraintsWithoutComment); dre != nil {
		if !strings.EqualFold(dre[1], "NULL") {
			defaultVal = strings.Trim(dre[1], "'")
		}
	}

	// 扫描约束关键字前必须先剥掉所有单引号字面量。
	// 否则 COMMENT '状态 NOT NULL AUTO_INCREMENT' 里的词会被当成真实约束,
	// 让列被错误标记为主键/自增/非空 —— 中文注释里出现这些词非常常见。
	constraintsUpper := strings.ToUpper(stripQuotedLiterals(cd.Constraints))

	if strings.Contains(constraintsUpper, "PRIMARY KEY") || strings.Contains(constraintsUpper, "PRIMARY_KEY") {
		isPK = true
	}
	if strings.Contains(constraintsUpper, "AUTO_INCREMENT") {
		isAutoIncr = true
	}
	if strings.Contains(constraintsUpper, "NOT NULL") {
		isNullable = false
	}

	// 构建 GORM tag
	gormTag := "column:" + cd.Name
	// FieldWithTypeTag: 仅当启用时才添加 type tag
	if conf.FieldWithTypeTag {
		gormTag += ";type:" + cd.SQLType
	}
	if isAutoIncr {
		// AUTO_INCREMENT 列必然是主键，primaryKey 放在 autoIncrement 前面
		gormTag += ";primaryKey;autoIncrement:true"
		isPK = true
	} else if isPK {
		gormTag += ";primaryKey"
	}
	if !isNullable && !isPK && !isAutoIncr {
		gormTag += ";not null"
	}
	// FieldWithIndexTag: 仅当启用时才添加 UNIQUE KEY 索引信息
	if conf.FieldWithIndexTag {
		for _, idx := range uniqueIdxs {
			gormTag += fmt.Sprintf(";uniqueIndex:%s,priority:%d", idx.IndexName, idx.Priority)
		}
	}
	if defaultVal != "" && !isPK && !isZeroDefault(goType, defaultVal) {
		gormTag += ";default:" + defaultVal
	}

	// 注释标签：始终保留 //，有内容时追加
	commentTag := "//"
	if comment != "" {
		commentTag = "// " + comment
	}

	// 处理 deleted_at 特殊类型
	if cd.Name == "deleted_at" && goType == "time.Time" {
		goType = "DeletedAt"
	}

	// FieldCoverable: 当字段具有默认值时生成指针
	// FieldNullable: 当字段可为空时生成指针（主键和 deleted_at 除外）
	switch {
	case conf.FieldCoverable && defaultVal != "" && !isZeroDefault(goType, defaultVal) && !isPK && cd.Name != "deleted_at":
		goType = "*" + goType
	case conf.FieldNullable && isNullable && !isPK && cd.Name != "deleted_at":
		goType = "*" + goType
	}

	return &Field{
		Name:         fieldName,
		Type:         goType,
		GORMTag:      gormTag,
		JSONTag:      "",
		CommentTag:   commentTag,
		IsPrimaryKey: isPK,
		ColumnName:   cd.Name,
	}
}

// ────── 共用 helper ──────

// toLowerCamel PascalCase 首字母转小写，用于 fileName 和 JSON tag（与 DB 路径一致）
func toLowerCamel(s string) string {
	if len(s) == 0 {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// compactSpaces 将连续空白压缩为单个空格
func compactSpaces(s string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

// unescapeSQLString 还原 SQL 字符串字面量中的转义：” → '，\' → '
func unescapeSQLString(s string) string {
	s = strings.ReplaceAll(s, "''", "'")
	s = strings.ReplaceAll(s, "\\'", "'")
	return s
}

// isZeroDefault 判断默认值是否是 Go 零值（无需写入 gorm default tag）
func isZeroDefault(goType, val string) bool {
	baseType := strings.TrimPrefix(goType, "*")
	switch baseType {
	case "int32", "int64", "uint32", "uint64", "float32", "float64":
		return val == "0"
	case "string":
		return val == ""
	case "bool":
		return val == "false"
	}
	return false
}

// ────── encoding ──────

// convertToUTF8 检测并转换 UTF-16(含 BOM)编码到 UTF-8。
// 无 BOM 的 UTF-16 不做猜测转换——前 4 字节的启发式会把纯 ASCII 的 UTF-8
// (每两字节第二字节恰为 0)误判为 UTF-16 LE,导致乱码。仅依赖 BOM 最稳妥,
// 请用户提供 UTF-8 或带 BOM 的 UTF-16 文件。
func convertToUTF8(data []byte) []byte {
	if len(data) >= 2 {
		// UTF-16 LE BOM: 0xFF 0xFE
		if data[0] == 0xFF && data[1] == 0xFE {
			return decodeUTF16LE(data[2:])
		}
		// UTF-16 BE BOM: 0xFE 0xFF
		if data[0] == 0xFE && data[1] == 0xFF {
			return decodeUTF16BE(data[2:])
		}
	}
	return data
}

// decodeUTF16LE 将 UTF-16 LE 字节解码为 UTF-8,正确处理代理对(surrogate pair)。
func decodeUTF16LE(data []byte) []byte {
	if len(data)%2 != 0 {
		return data
	}
	u16 := make([]uint16, 0, len(data)/2)
	for i := 0; i < len(data); i += 2 {
		u16 = append(u16, uint16(data[i])|uint16(data[i+1])<<8)
	}
	return []byte(string(utf16.Decode(u16)))
}

// decodeUTF16BE 将 UTF-16 BE 字节解码为 UTF-8,正确处理代理对(surrogate pair)。
func decodeUTF16BE(data []byte) []byte {
	if len(data)%2 != 0 {
		return data
	}
	u16 := make([]uint16, 0, len(data)/2)
	for i := 0; i < len(data); i += 2 {
		u16 = append(u16, uint16(data[i])<<8|uint16(data[i+1]))
	}
	return []byte(string(utf16.Decode(u16)))
}
