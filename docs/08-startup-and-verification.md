# 启动与验证流程

本篇是 `owl` 新子系统的**验收闭环**。目标不是解释架构，而是让开发者或 AI 在生成代码后，能快速判断“这个子系统到底有没有真的跑起来”。

建议配合 [07-minimal-subapp-template.md](07-minimal-subapp-template.md) 使用。

## 第一次启动前的检查

### 1. `go.mod`

- 已声明 `require bit-labs.cn/owl`
- 本地开发时已配置 `replace bit-labs.cn/owl => ../owl`

### 2. SubApp 接线

- `main.go` 调用了 `owl.NewApp(&your.SubApp{}).WebShell()`
- SubApp 结构体存在 `app foundation.Application`
- `ServiceProviders()` 至少包含 `router.RouterServiceProvider` 与 `db.DBServiceProvider`
- `Binds()` 已注册 `NewXxxHandle`、`NewXxxService`、`NewXxxRepository`
- `RegisterRouters()` 已调用 `route.InitApi(i.app, i.Name())`
- SubApp 已实现 `RegisterMigrate()` / `BeforeMigrate` / `AfterMigrate`

### 3. 先用 sqlite 跑通

对于第一个版本，建议使用 sqlite，减少环境依赖：

```yaml
# conf/database.yaml
driver: sqlite
host: my-order.db
database: main
```

注意：`DBServiceProvider` 在 sqlite 模式下会把 `host` 拼到 `GetConfigPath()` 下，因此数据库文件通常落在 `conf/my-order.db`。

## 启动命令

### 第一次拉依赖

```bash
go mod tidy
```

### 启动服务

```bash
go run .
```

如果项目不是在仓库根目录启动，也可以显式指定：

```bash
go run ./...
```

## 首次启动后应该发生什么

### 1. 自动生成 `conf/`

若项目下没有 `conf/`，框架会创建该目录，并根据 Provider 的 `Conf()` 自动生成缺失配置文件。

在最小 Web 子系统里，通常会看到：

- `conf/app.yaml`
- `conf/database.yaml`
- `conf/router.yaml`
- `conf/log.yaml`

### 2. 自动建表

如果 SubApp 实现了 `RegisterMigrate()`，第一次启动后框架会对每个 Model 分别 AutoMigrate，并在 `storage/migrate_hash.txt` 中按行记录 `类型名=hash`；后续启动仅迁移结构有变化的 Model。

例如最小模板应出现：

- `order_order`

### 3. HTTP 服务监听

若 `conf/router.yaml` 使用默认配置，通常可访问：

- `http://127.0.0.1:8080/health`
- `http://127.0.0.1:8080/api/v1/orders`

## Smoke Test

以下以最小订单模板为例。

### 1. 健康检查

```bash
curl "http://127.0.0.1:8080/health"
```

期望：

- 返回 HTTP 200
- 表示 `router.RouterServiceProvider` 已成功启动

### 2. 创建一条订单

```bash
curl -X POST "http://127.0.0.1:8080/api/v1/orders" ^
  -H "Content-Type: application/json" ^
  -d "{\"orderNo\":\"SO20260319001\",\"title\":\"首单\",\"status\":1}"
```

期望：

- 返回 `success: true`
- 数据库表 `order_order` 中出现一条记录

### 3. 查询列表

```bash
curl "http://127.0.0.1:8080/api/v1/orders?page=1&pageSize=10"
```

期望：

- 返回 `success: true`
- `data.list` 中包含刚创建的订单

## 失败时的排查顺序

### 服务起不来

优先排查：

1. `ServiceProviders()` 是否缺少 `router.RouterServiceProvider`
2. `Binds()` 是否漏注册某个构造函数
3. `SubApp` 是否缺少 `app foundation.Application`

### 服务启动了但接口 404

优先排查：

1. `RegisterRouters()` 是否调用了 `route.InitApi`
2. `InitApi` 中是否真的注册了目标路由
3. 路由是否注册到了 `/api/v1` 分组

### 写入接口报数据库错误

优先排查：

1. SubApp 是否实现了 `RegisterMigrate()` 并返回目标 model
2. `database.Models()` 是否包含目标 model
3. `conf/database.yaml` 是否指向正确数据库

### 启动后没看到配置文件

优先排查：

1. 当前使用的是 `basePath` 还是 `runDir`
2. 是否在错误的工作目录下运行
3. 可执行文件目录与项目目录是否不一致

## 交付前最小验收标准

- [ ] 服务能成功启动，不 panic
- [ ] `conf/` 自动生成成功
- [ ] 迁移执行成功，数据库里能看到新表
- [ ] 至少一条创建接口和一条列表接口能成功返回
- [ ] 路由路径、请求方法、返回结构与文档一致

## 完成定义

- 看完本文后，开发者或 AI 能按固定步骤判断一个新子系统是否已经真正可用。
- 即使失败，也能按“启动失败 / 404 / 数据库错误 / 配置目录错误”的顺序快速定位问题。
