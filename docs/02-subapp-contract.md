# SubApp 契约

## 接口定义

子应用必须实现 `owl.SubApp`，定义在 `application.go`：

```go
type SubApp interface {
    Name() string
    RegisterRouters()
    ServiceProviders() []foundation.ServiceProvider
    Binds() []any
    Menu() []*router.Menu
    Commands() []*cobra.Command
    Bootstrap()
}
```

## 各方法职责

| 方法 | 职责 |
|------|------|
| **Name()** | 子应用标识，用于路由权限前缀等（如 `admin`）。 |
| **RegisterRouters()** | 仅在 WebShell 下调用，注册本子应用所有 HTTP 路由。 |
| **ServiceProviders()** | 返回本子应用依赖的 Provider（如 router、db、redis、jwt）。 |
| **Binds()** | 返回构造函数列表（Repository、Service、Handle 等），供 DI 注册，不是实例。 |
| **Menu()** | 返回本子应用的菜单树，仅 WebShell 下使用。 |
| **Commands()** | 子应用专属 Cobra 命令，可为 nil 或空。 |
| **Bootstrap()** | 所有依赖就绪后执行，适合做迁移、Seeder、事件监听器注册等。 |

## 必须持有的 app 字段

**SubApp 结构体必须包含名为 `app` 的字段，类型为 `foundation.Application`。**

框架通过反射注入：

```go
// application.go
func (i *Application) injectAppInstance(target any) {
    field := reflect.Indirect(reflect.ValueOf(target)).FieldByName("app")
    // ...
    fieldValue.Set(reflect.ValueOf(i))
}
```

若没有 `app` 字段，运行时会 panic。Service Provider 同样依赖 `FieldByName("app")` 注入，自定义 Provider 也需保留该字段。

### 最小可运行骨架

```go
package admin

import (
    "bit-labs.cn/owl"
    "bit-labs.cn/owl/contract/foundation"
    "bit-labs.cn/owl/provider/router"
    "github.com/spf13/cobra"
)

var _ owl.SubApp = (*SubAppAdmin)(nil)

type SubAppAdmin struct {
    app foundation.Application
}

func (i *SubAppAdmin) Name() string { return "admin" }

func (i *SubAppAdmin) RegisterRouters() {
    // 例如：route.InitApi(i.app, i.Name())
}

func (i *SubAppAdmin) ServiceProviders() []foundation.ServiceProvider {
    return []foundation.ServiceProvider{
        // &router.RouterServiceProvider{},
        // &db.DBServiceProvider{},
        // ...
    }
}

func (i *SubAppAdmin) Binds() []any {
    return []any{
        // NewXxxHandle, NewXxxService, NewXxxRepository, ...
    }
}

func (i *SubAppAdmin) Menu() []*router.Menu {
    return nil // 或 route.InitMenu()
}

func (i *SubAppAdmin) Commands() []*cobra.Command {
    return nil
}

func (i *SubAppAdmin) Bootstrap() {
    // 迁移、Seeder、listener.Init(i.app) 等
}
```

## 完成定义

- 能列出 SubApp 的七个方法及其调用时机。
- 能说明为何必须存在 `app foundation.Application` 字段及注入方式。
- 能写出一个最小可编译、可挂到 `NewApp()` 的 SubApp 骨架。
