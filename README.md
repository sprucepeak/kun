# kun — A CLI tool for building go applications.

### 地势坤，君子以厚德载物

`kun`（坤）是一个基于 Golang 的工程化应用脚手架与脚手架 CLI 工具，整合了 Go 生态中成熟、高效的优秀开源库，旨在帮助开发者快速构建高性能、可扩展、易维护的企业级微服务与 Web 应用。

> [!TIP]
> **💡 规范变更提醒**
>
> 1.3.0 之前项目使用 `controller` 命名控制器层，现已全面统一规范为 `handler`，请在阅读和扩展业务时注意保持一致。

---

## 核心技术栈

- **Web 框架**: [Gin](https://github.com/gin-gonic/gin) — 极速 HTTP 路由与中间件
- **ORM 引擎**: [GORM](https://github.com/go-gorm/gorm) — 强大的关系型数据库对象关系映射
- **依赖注入**: [Wire](https://github.com/google/wire) — Google 编译期强类型依赖注入
- **配置管理**: [Viper](https://github.com/spf13/viper) — 动态配置加载与环境隔离
- **结构化日志**: [Zap](https://github.com/uber-go/zap) — 极速结构化日志，内置 OpenTelemetry 分布式追踪与调用栈穿透
- **安全认证**: [Golang-jwt](https://github.com/golang-jwt/jwt) — HS384 双 Token 轮换、单端互斥与重放熔断
- **缓存组件**: [Go-redis](https://github.com/redis/go-redis) — Redis 单机/集群客户端与单槽 Lua 原子脚本
- **任务与消息**: [Asynq](https://github.com/hibiken/asynq) / [Kafka](https://github.com/IBM/sarama) — 异步延时任务队列与分布式事件流
- **接口文档**: [Swaggo](https://github.com/swaggo/swag) — 声明式 Swagger API 接口文档生成

---

## 架构特性

* **超低学习成本与极速定制**：封装 Gopher 最熟悉的主流基础库，代码结构开箱即用，易于按需自由定制。
* **高性能与极致轻量**：坚持纯 Go 实现，全内置数据库驱动**完全免 CGO 依赖**，支持极速交叉编译与瘦身部署。
* **高可靠与企业级安全**：JWT 采用 CSPRNG 密码学安全随机数、双密钥物理隔离、Redis Cluster 单槽路由及全链路调用栈穿透日志定位。
* **模块化与工程解耦**：基于 Wire 依赖注入与事件发布/订阅模式，职责清晰分明，高并发下从容扩展。

---

## 简洁分层架构

kun 采用经典清晰的领域分层架构，配合 Wire 实现编译期自动装配：

![layout](layout.png)

---

## 目录结构说明

```
.
├── cmd/                           应用程序的主要入口
│   ├── broker/                    异步调度服务器入口（仅在 Advanced 布局中提供）
│   │   ├── main.go                Wire DI 启动入口，信号优雅关闭
│   │   └── wire/                  Wire 依赖注入与应用装配
│   └── server/                    HTTP 服务的入口
│       ├── main.go                主函数，启动 HTTP 服务
│       └── wire/                  Wire 依赖注入与应用装配
│           ├── app.go             HTTP 应用装配（Gin 模式、中间件、路由与资源优雅回收）
│           ├── wire.go            声明 Server 全栈分层依赖注入规则
│           └── wire_gen.go        Wire 自动生成的装配代码
├── config/                        配置文件目录
│   ├── local.yml                  本地开发环境配置文件
│   └── release.yml                生产环境配置文件
├── internal/                      内部核心业务代码（禁止外部包导入）
│   ├── global/                    全局常量与枚举
│   │   └── constant.go            上下文 Key 与路由前缀常量定义
│   ├── handler/                   HTTP 处理器层（参数校验、协议转换、调用 Service）
│   │   └── serverDI.go            Wire DI 注册全部 Handler
│   ├── middleware/                HTTP 中间件
│   │   ├── auth.go                JWT 授权中间件（支持强制/可选/写入模式）
│   │   ├── cors.go                跨域资源共享中间件
│   │   ├── metrics.go             Prometheus 监控指标收集中间件
│   │   ├── ratelimit.go           单机 / Redis 分布式限流中间件
│   │   └── recovery.go            Panic 异常捕获与堆栈保护
│   ├── repository/                存储与数据持久化层
│   │   ├── cache/                 缓存访问层（Local + Redis 二级缓存）
│   │   │   ├── keys.go            缓存 Keys 统一模板定义
│   │   │   └── local.go           线程安全本地内存缓存封装
│   │   ├── db/                    数据库访问层
│   │   │   ├── demo.go            自定义数据库查询与复杂业务事务
│   │   │   └── demo_gen.go        自动生成的基础 CRUD 样板代码
│   │   └── serverDI.go            Wire DI 注册全部 Repository
│   ├── router/                    路由注册层
│   │   ├── router.go              全局通用路由（404/健康探针/Metrics）
│   │   ├── serverDI.go            Wire DI 注册全部路由
│   │   └── v0/                    版本化业务路由
│   └── service/                   核心业务逻辑层
│       ├── svc/                   核心业务逻辑编排（事务、领域计算）
│       │   └── context.go         业务公共上下文（配置、DB、Redis 等）
│       └── serverDI.go            Wire DI 注册全部 Service
├── pkg/                           跨项目公共工具包（加解密、JWT、网络、配置、日志等）
├── README.md                      项目说明文档
├── go.mod                         Go 模块依赖定义文件
└── go.sum                         Go 模块版本校验文件
```

---

## 环境要求

* **Go**: `1.26.8` 或更高版本
* **Git**
* **Docker** (可选，用于本地一键拉起依赖中间件)
* **MySQL 5.7+ / PostgreSQL / SQLite / ClickHouse** (可选)
* **Redis** (可选)
* **Mockgen** (可选，执行 `kun mock` 生成单测 Mock 桩代码时需要): `go install go.uber.org/mock/mockgen@latest`

---

## 安装与快速上手

### 1. 安装 kun CLI 工具

推荐使用 `-ldflags="-s -w"` 链接压缩安装，体积直减 40%，开箱即用：

```bash
go install -ldflags="-s -w" github.com/sprucepeak/kun@latest
```

> [!TIP]
> 国内用户可配置 GOPROXY 加速：
>
> ```bash
> go env -w GO111MODULE=on
> go env -w GOPROXY=https://goproxy.cn,direct
> ```
>
> 若安装后提示找不到 `kun` 命令，请将 `$GOPATH/bin`（或 `%USERPROFILE%\go\bin`）添加至系统的 PATH 环境变量中。

### 2. 内置全纯 Go 数据库驱动说明

kun 内置支持全主流数据库逆向与代码生成，**所有驱动均为纯 Go 实现，完全无需 CGO**，支持静态编译与无缝跨平台交叉构建：

| 驱动                 | 默认支持 | 驱动库                                        | CGO 依赖 |
| :------------------- | :------: | :-------------------------------------------- | :------: |
| **MySQL**      |    ✅    | 官方纯 Go 驱动`gorm.io/driver/mysql`        |  ❌ 无  |
| **PostgreSQL** |    ✅    | 官方纯 Go 驱动`gorm.io/driver/postgres`     |  ❌ 无  |
| **SQLite**     |    ✅    | 纯 Go 驱动`github.com/glebarez/sqlite`      |  ❌ 无  |
| **ClickHouse** |    ✅    | 官方 Native 驱动`gorm.io/driver/clickhouse` |  ❌ 无  |

---

## 创建新项目 (`kun new`)

```bash
# 创建新项目（推荐新手优先选择 Advanced Layout）
kun new projectName

# 支持通过 Git 指定远程模板
kun new projectName -g https://github.com/sprucepeak/kun.git
```

> kun 内置了两种不同定位的工程模板：
>
> - **基础模板 (Basic Layout)**: 极简目录架构，仅包含基础 CRUD、日志与鉴权，适合资深开发者快速起步。
> - **高级模板 (Advanced Layout)**: 包含双 Token 轮换认证、单端登录互斥、Redis 缓存击穿防护、Kafka + Asynq 异步调度中心、WebSocket、Excel 流式导出与 Swagger 等全套最佳实践。

---

## 业务代码生成器 (`kun gen`)

kun 提供了强大的脚手架代码生成命令，帮助开发者告别低效手写胶水代码：

```bash
# 1. 单层业务组件生成
kun gen rt user                                         # 生成 router 路由文件
kun gen hdl user                                        # 生成 handler 处理器
kun gen svc user                                        # 生成 service 业务服务
kun gen cache user                                      # 生成 cache 缓存层

# 2. 复合业务层一键生成
kun gen hs user                                         # 一键生成 Handler + Service
kun gen crud user                                       # 一键生成 Router + Handler + Service

# 3. 数据库全自动逆向代码生成（自动创建 Model 结构体与 Repository CRUD 样板代码）
kun gen db "root:123456@tcp(127.0.0.1:3306)/dbname" "*"    # MySQL 全库表逆向
kun gen db "postgres://user:pwd@localhost:5432/dbname" "*" # PostgreSQL 逆向
kun gen db "data.db" "*"                                   # SQLite 本地库逆向
kun gen db "clickhouse://127.0.0.1:9000/dbname" "*"        # ClickHouse 逆向
kun gen db "schema.sql" "*"                                # 本地 DDL SQL 脚本直接逆向
```

> [!TIP]
> **💡 业务占位规范：`// TODO: add` 标识**
>
> 自动生成的各层代码中统一保留了 `// TODO: add ... and delete this line` 待办标识：
>
> - **一键检索**：在 IDE 中全局搜索 `// TODO: add` 即可一站式定位需要实现具体业务逻辑、自定义查询条件或结构体扩展的代码点；
> - **避免遗漏**：在实际编写完对应业务逻辑前，请保留该注释，避免遗漏关键步骤。

---

## 单元测试 Mock 生成器 (`kun mock`)

`kun mock` 深度整合了 `go:generate` 与 `mockgen` 工具链，支持一键扫描源码中的 `//go:generate` 指令并自动生成单测打桩代码：

```bash
# 1. 全局扫描生成（默认执行当前项目下所有 //go:generate 指令）
kun mock

# 2. 针对指定目录或单个接口文件生成 Mock
kun mock ./internal/service/svc/...
kun mock ./internal/service/svc/demo.go

# 3. 正则过滤执行（仅执行包含 mockgen 的指令，跳过其他生成器）
kun mock -r mockgen

# 4. 调试与排错模式（输出详细日志并打印底层执行的实际命令）
kun mock -v -x
```

> [!TIP]
> **💡 前置依赖与编写规范**
>
> - **前置工具**：使用前请确保已安装 `mockgen`（`go install go.uber.org/mock/mockgen@latest`）。若未安装，`kun mock` 会给出直接安装提示。
> - **指令格式**：在待 Mock 接口定义上方添加指令，`//` 与 `go:generate` 之间**严禁有空格**：
>   ```go
>   //go:generate mockgen -source=./demo.go -destination=../../../test/mocks/service/demo.go -package mock_service
>   ```

---

## 代码质量与安全门禁 (`kun check`)

`kun check` 提供了工业级的分级质量门禁（Quality Gate），采用**“日常极速模式 + 深度/供应链安全审计”**的最佳实践：

| 维度                           | 核心工具          |     耗时     | 作用与覆盖范围                                                                                      |
| :----------------------------- | :---------------- | :-----------: | :-------------------------------------------------------------------------------------------------- |
| **日常极速静态分析**     | `golangci-lint` | **~2s** | 内存共享 AST 极速并发扫描：覆盖`govet`、`errcheck`、`gosec`(代码安全)、`nilerr` 等 50+ 规则 |
| **并发竞态动态检测**     | `go test -race` | **~3s** | 运行时多协程读写内存冲突探测（Data Race），自动化单测非阻塞执行                                     |
| **第三方依赖 CVE 审计**  | `govulncheck`   |     ~10s     | 联网比对 Go 官方实时漏洞库，符号级分析代码是否实际调用了漏洞依赖（通过`--cve` 触发）              |
| **跨函数深度空指针分析** | Uber`nilaway`   |     ~30s     | 跨函数 Inter-procedural SSA 数据流深度推导潜在`nil panic`（通过 `--nil` 触发）                  |

```bash
# 1. 日常极速检查（默认执行：golangci-lint + race test，秒级完成）
kun check

# 2. 针对指定子目录进行日常检查
kun check ./internal/service/...

# 3. 深度全量检查（包含供应链 CVE 漏洞审计与 SSA 深度空指针分析）
kun check --deep

# 4. 单项/专项检查
kun check --cve               # 仅执行第三方依赖已知 CVE 漏洞审计 (govulncheck)
kun check --nil               # 仅执行跨函数 SSA 深度空指针分析 (nilaway)
kun check --lint              # 仅执行 golangci-lint
kun check --race              # 仅执行并发竞态单测

# 5. 离线/受限网络环境模式（不发起自动安装，缺失工具自动跳过）
kun check --no-install

# 6. 主动一键预装所有外部检查工具链
kun check init
```

> [!TIP]
> **💡 首次自动就绪与自愈机制**
>
> - **无缝就绪**：`kun check` 采用动态探测机制，首次运行时若缺少 `golangci-lint`、`govulncheck` 或 `nilaway`，将自动静默安装并即刻执行；后续运行零安装耗时。
> - **优雅降级**：若由于离线网络或未配置 `GOPROXY` 导致外部工具下载失败，程序不会崩溃，会自动将其标记为 `SKIPPED` 并继续完成已有检查。

---

## Swagger 接口文档生成 (`kun swag`)

`kun swag` 封装了 `swag` 声明式文档生成工具，解析代码注释并自动生成 Swagger API 静态文档文件。

> [!IMPORTANT]
> **💡 架构约定与 `swagger` 文件夹校验**
>
> - **输出目录约束**：在 kun 工程架构中，Swagger 文档代码统一放置在根目录下的 **`swagger/`** 目录（代码内部由 `internal/router/router.go` 显式导入 `import "{projectName}/swagger"` 供路由直接注册编译）。
> - **目录防呆检查**：执行 `kun swag` 时会**自动检测当前项目根目录下是否存在 `swagger` 文件夹**。若未检测到，将输出友好错误及创建指引（`mkdir swagger`），防止文档生成到错误位置导致无法编译进程序。

```bash
# 1. 默认一键生成（自动寻找入口 cmd/server/main.go 并输出至 ./swagger）
kun swag

# 2. 手动指定通用 API 入口文件与输出目录
kun swag -g cmd/server/main.go -o ./swagger

# 3. 首次未安装 swag 时将自动静默安装，离线模式可用 --no-install
kun swag --no-install
```

---

## 启动与编译

```bash
# 1. 快速启动开发调试
kun run

# 2. 启动指定入口服务（如 Advanced 中的 Broker 消息调度服务器）
kun run ./cmd/broker

# 3. 编译 Wire 依赖注入代码（生成 wire_gen.go）
kun wire
kun wire all  # 全量递归编译

# 4. 生成单元测试 Mock 桩代码
kun mock

# 5. 全面代码质量与安全检查（vet + nilaway + errcheck + govulncheck + race）
kun check

# 6. 生成 Swagger 接口文档（输出至 ./swagger 目录）
kun swag
```

---

## 开源协议与致谢

- **开源协议**: 本项目基于 [MIT License](LICENSE) 协议开源。
- **鸣谢**: 本项目借鉴了 Go 开源生态中 [nunu](https://github.com/go-nunu/nunu) 的优秀设计思想，并根据实际企业级高并发开发经验进行了全方位加固与重构优化。
