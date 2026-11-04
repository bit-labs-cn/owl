# SubApp 契约

## 接口定义

子应用必须实现 `owl.SubApp`，定义在 `application.go`：

```go
type SubApp interface {
    Name() string
    RegisterRouters()
    ServiceProviders() []foundation.ServiceProvider
    Binds() []any
    Bootstrap()
    RegisterMenus() []*router.Menu
    RegisterCommands() []*cobra.Command
    RegisterMigrate() []any
    BeforeMigrate(db *gorm.DB) error
    AfterMigrate(db *gorm.DB) error
}
```

## 各方法职责

| 方法 | 职责 |
|------|------|
| **Name()** | 子应用标识，用于路由权限前缀等（如 `admin`）。 |
| **RegisterRouters()** | 仅在 WebShell 下调用，注册本子应用所有 HTTP 路由。 |
| **ServiceProviders()** | 返回本子应用依赖的 Provider（如 router、db、redis、jwt）。 |
| **Binds()** | 返回构造函数列表（Repository、Service、Handle 等），供 DI 注册，不是实例。 |
| **RegisterMenus()** | 返回本子应用的菜单树，仅 WebShell 下使用。 |
| **RegisterCommands()** | 子应用专属 Cobra 命令，可为 nil 或空。 |
| **Bootstrap()** | 所有依赖就绪后执行，适合做 Seeder、事件监听器注册等（不含表结构迁移）。 |
| **RegisterMigrate()** | 返回待 AutoMigrate 的 GORM model 列表。 |
| **BeforeMigrate / AfterMigrate** | 统一迁移前后钩子（始终执行）。 |

## 可选：登记系统配置

不改 `SubApp` 接口。需要配置项的子系统额外实现 `setting.Registrar`（`bit-labs.cn/owl/provider/setting`）：

```go
func (i *SubAppOrder) RegisterConfigs() setting.Catalog {
    return setting.Catalog{
        Title: "订单",
        Groups: []setting.Group{{
            Key:   "trade",
            Title: "交易",
            Items: []setting.Item{{
                Key: "closeUnpaidMinutes", Title: "未支付关闭时间",
                Type: setting.TypeInt, Default: "30", Min: 1, Max: 1440,
            }},
        }},
    }
}
```

框架在注册路由之前调用它，Web 和命令行都会收集。标识用 `Name()`，页面上的名称用 `Catalog.Title`。配置项的键在同一子系统内唯一。分组最多两级：系统 / 一级 / 二级。一级可以只放配置项，也可以再放一组二级分组；二级只能放配置项。同一组不能同时放配置项和子分组。类型、标题、说明、默认值、是否公开和取值范围都写在配置项上。类型可以是 `string`、`bool`、`int`、`image`。`image` 的值是 `data:image/png;base64,...` 这种 base64，解码后不能超过 200KB，空字符串表示不另存图片。没实现这个接口的子系统不会出现在配置页。

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
    "gorm.io/gorm"
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

func (i *SubAppAdmin) RegisterMenus() []*router.Menu {
    return nil // 或 route.InitMenu()
}

func (i *SubAppAdmin) RegisterCommands() []*cobra.Command {
    return nil
}

func (i *SubAppAdmin) Bootstrap() {
    // Seeder、listener.Init(i.app) 等
}

func (i *SubAppAdmin) RegisterMigrate() []any { return nil }

func (i *SubAppAdmin) BeforeMigrate(db *gorm.DB) error { return nil }

func (i *SubAppAdmin) AfterMigrate(db *gorm.DB) error { return nil }
```

## 完成定义

- 能列出 SubApp 的接口方法及其调用时机。
- 能说明为何必须存在 `app foundation.Application` 字段及注入方式。
- 能写出一个最小可编译、可挂到 `NewApp()` 的 SubApp 骨架。
