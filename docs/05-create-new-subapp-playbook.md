# 从 0 到 1 创建新子系统

本 playbook 描述基于 Owl 从零创建一个**独立项目**（新仓库、独立 `main.go` 与 `app` 包）的标准步骤。

如果你想直接复制一个可运行骨架，请同时阅读：

- [07-minimal-subapp-template.md](07-minimal-subapp-template.md)
- [08-startup-and-verification.md](08-startup-and-verification.md)

## 前置条件

- 已阅读 [01-application-lifecycle](01-application-lifecycle.md)、[02-subapp-contract](02-subapp-contract.md)、[03-provider-and-di](03-provider-and-di.md)。
- 本地有 `owl` 仓库，且新项目的 `go.mod` 能通过 `require` + 本地 `replace` 引用 `bit-labs.cn/owl`。

## 步骤 1：新建独立项目

- 创建新仓库，根目录包含 `main.go`、`go.mod`。
- `go.mod` 模块名自定（如 `bit-labs.cn/my-biz`），依赖 `bit-labs.cn/owl`，必要时 `replace bit-labs.cn/owl => ../owl`。

## 步骤 2：确定 app 包名

- 在项目根下创建 `app/` 目录，包名与业务一致（如 `order`、`biz`）。
- SubApp 的 `Name()` 返回值建议与包名对应（如 `"admin"`、`"order"`），用于路由前缀与权限标识。

## 步骤 3：创建 SubApp 骨架

- 在 `app/` 下创建 `app.go`，定义实现 `owl.SubApp` 的结构体（如 `SubAppOrder`）。
- **必须**包含字段：`app foundation.Application`。
- 实现全部接口方法：`Name`、`RegisterRouters`、`ServiceProviders`、`Binds`、`Menu`、`Commands`、`Bootstrap`。
- `ServiceProviders()` 至少返回本子应用需要的 Provider（如 `router.RouterServiceProvider`、`db.DBServiceProvider`、`redis.RedisServiceProvider` 等）。
- `Binds()` 可先返回 `nil` 或空切片，后续按资源递增。

### 常用 Provider 组合

| 子系统类型 | 推荐 Provider |
|------|------|
| 最小公开 CRUD API | `router.RouterServiceProvider`、`db.DBServiceProvider` |
| 带分布式锁的 CRUD API | 上述 + `redis.RedisServiceProvider` |
| 带 RBAC 的后台子系统 | 上述 + `permission.GuardProvider` + 自定义 `jwt` Provider |
| 仅命令行子系统 | 可不注册 `router.RouterServiceProvider`，直接用 `ConsoleShell()` |

## 步骤 4：编写 main.go

- 仅注册本项目的 SubApp，例如：  
  `owl.NewApp(&order.SubAppOrder{}).WebShell()`  
- import 使用本项目模块名下的 app 包（如 `admin "bit-labs.cn/owl-admin/app"`）。

## 步骤 5：实现路由注册

- 在 `app/route/` 下实现路由初始化（如 `api.go`），在 `RegisterRouters()` 中调用，例如：`route.InitApi(i.app, i.Name())`。
- `InitApi` 内通过 `app.Invoke(func(engine *gin.Engine, ...) { ... })` 取得 `*gin.Engine` 及需要的 Handle，使用 `router.NewRouteInfoBuilder(appName, handle, gv1, router.MenuOption{...})` 注册路由，每条路由链式 `.Name("中文").Build()`。
- 若需鉴权，在路由分组上挂载权限中间件（如 Casbin + JWT），并在 `ServiceProviders()` 中注册对应 Provider。

## 步骤 6：按资源补齐分层并注册 Binds

对每个业务资源（如“订单”）：

1. **model**：在 `app/model/` 定义表结构，实现 `TableName()`，使用框架提供的 `db.BaseModel` 等。
2. **repository**：在 `app/repository/` 定义接口与实现，提供 `WithContext`、查询/保存等方法，构造函数返回接口类型。
3. **service**：在 `app/service/` 实现业务逻辑，构造函数依赖 Repository（及可选 Redis Locker、Validator）。
4. **handle**：在 `app/handle/v1/` 实现 HTTP 处理，实现 `router.Handler`（`ModuleName()`），方法内 Bind → Service → `router.Success`/`router.Fail`。
5. 在 SubApp 的 **Binds()** 中追加 `NewXxxRepository`、`NewXxxService`、`NewXxxHandle`。
6. 在 **route** 中为该 Handle 使用 `NewRouteInfoBuilder` 注册路由与菜单。
7. 在 **database**：在 `app/database/auto_migrate_gen.go` 的 `Migrate(db)` 中对该 model 执行 `AutoMigrate(&Xxx{})`，并在 **Bootstrap()** 中调用 `database.Migrate(migDB)`。

## 步骤 7：配置与首次运行

- 首次运行后，框架会根据各 Provider 的 `Conf()` 在 `conf/` 下生成缺失的配置文件。
- 按需修改 `conf/database.yaml`、`conf/redis.yaml`、`conf/router.yaml` 等；敏感项可用环境变量覆盖（参见 `provider/conf/ENV_VARIABLES.md`）。

### 最低成本的第一次运行方式

建议第一版先用 sqlite 跑通：

```yaml
driver: sqlite
host: my-order.db
database: main
```

这样可以先验证骨架、路由和迁移，不必先准备 pgsql/mysql/redis。

## 完成定义

- 能独立新建一个仅含一个 SubApp 的 Go 项目，能启动 WebShell 并访问至少一个已注册路由。
- 能说明新增一个“资源”需要在 model、repository、service、handle、Binds、route、Migrate 中分别做哪些事。
- 能按 [08-startup-and-verification.md](08-startup-and-verification.md) 完成一次完整的启动与 smoke test。
