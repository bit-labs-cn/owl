# 路由、菜单、配置与运行目录

## 路由与 Handler 契约

- **Handler**：业务控制器需实现 `router.Handler`，定义在 `provider/router/handle.go`：

```go
type Handler interface {
    ModuleName() (en string, zh string)
}
```

- 路由注册使用 `router.NewRouteInfoBuilder(appName, handle, routerGroup, menuOption)`，再链式调用 `.Get/.Post/.Put/.Delete( path, accessLevel, handle ).Name("中文").Build()`。  
- 权限标识形如 `appName:moduleEn:方法名`，由 `RouterInfoBuilder` 根据 `Handler.ModuleName()` 与函数名生成。  
- 路由信息会写入全局表（`RegisterRoute`），供权限中间件、操作日志、接口列表等使用。

详见 `provider/router/router.go`（`NewRouteInfoBuilder`、`AccessLevel`、`Build`、`RegisterRoute`、`GetAllRoutes`）。

## 菜单

- 每个子应用通过 `RegisterMenus()` 返回 `[]*router.Menu`，框架在 WebShell 下收集后调用 `menuManager.AddMenu(i.menus...)`，再执行各 SubApp 的 `Bootstrap()`。  
- 单模块的菜单可由 `RouterInfoBuilder` 的 `MenuOption` 与 `GetMenu()` 得到，再在子应用内组装成树并在 `RegisterMenus()` 中返回。  
- 菜单结构见 `provider/router/menu.go`（`Menu`、`Meta`、`MenuType` 等）。

### 宿主定制菜单 Meta

框架在 `AddMenu` 之后执行各 SubApp 的 `Bootstrap()`，因此宿主可在 `Bootstrap()` 中注入 `*router.MenuRepository`，按路由 **Name**（不是 path）修改已注册菜单的 Meta：

```go
func (i *SubAppHost) Bootstrap() {
    _ = i.app.Invoke(func(menuRepo *router.MenuRepository) {
        menuRepo.ChangeName("Workorder", "OA引擎")
        menuRepo.ChangeMeta("SomeMenu", func(meta *router.Meta) {
            meta.Icon = "ep:office-building"
        })
    })
}
```

- `ChangeName(menuName, newTitle)`：修改 `Meta.Title`
- `ChangeMeta(menuName, mutator)`：修改任意 Meta 字段
- 修改在 `MenuSaveServiceProvider` 写库前生效，会持久化到数据库

## 配置与配置文件生成

- 配置目录为 `GetConfigPath()`，通常为项目下的 `conf/`，路径受 `inferDir("conf")` 影响（先 basePath，再 runDir）。  
- 启动时扫描 `conf/`，加载各 yaml/json；环境变量通过 Viper 覆盖，规则见 `provider/conf/ENV_VARIABLES.md`（如 `DATABASE_*`、`REDIS_*`）。  
- 各 Provider 的 `Conf()` 返回的默认内容会在首次缺失时写入 `conf/`，避免手写模板。

## 环境变量与 .env

- 支持 `.env.local`、`.env.{环境名}`、`.env` 等，由 `loadEnvFiles()` 按优先级加载。  
- 环境名由 `APP_ENV` 或 `ENVIRONMENT` 等决定。

## 运行目录规则

- **basePath**：可执行文件所在目录。  
- **runDir**：当前工作目录。  
- `GetConfigPath()`、`GetStoragePath()` 等通过 `inferDir` 先拼 basePath，若目录不存在再拼 runDir。  
- 部署时若以服务方式运行，工作目录可能与可执行文件目录不同，需确认 `conf/`、`storage/` 的实际落点。

## 完成定义

- 能说明 Handler 需实现 `ModuleName()`，以及路由、权限标识、菜单的注册流程。
- 能说明配置目录的来源与 Provider 生成默认配置的机制。
- 能说明 basePath 与 runDir 对配置和存储目录的影响及部署注意点。
