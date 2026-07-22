# Service Provider 与依赖注入

## Service Provider 契约

定义在 `contract/foundation/service_provider.go`：

```go
type ServiceProvider interface {
    Register()               // 注册服务到容器
    Boot()                    // 启动阶段初始化（路由、视图等）
    Conf() map[string]string  // 返回 文件名 -> 默认配置内容，用于生成 conf/ 下文件
    Description() string     // 服务描述
}
```

Provider 结构体同样需要 `app foundation.Application` 字段，框架会通过 `injectAppInstance(serviceProvider)` 注入。

## Register 与 Conf

- **Register()**：在其中通过 `app.Register(func(...) T { ... })`（即 `app.Provide(...)`）向 dig 注册构造函数。其他组件通过依赖该类型自动解析。
- **Conf()**：返回 `map[string]string`，key 为配置文件名（如 `jwt.yaml`），value 为默认内容。框架在首次启动时若 `conf/` 下无该文件则写入；若已存在则仅做键校验（见 `conf.ValidateConfigKeys`）。

## Boot

所有 SubApp 与 Provider 的 `Register()` 和子应用的 `RegisterRouters()`、`RegisterMenus()`、`Bootstrap()` 执行完后，再统一执行每个 Provider 的 `Boot()`。适合做依赖已就绪的收尾初始化。

## Binds() 组织方式

SubApp 的 `Binds()` 返回的是**构造函数**（如 `NewUserHandle`、`NewUserService`、`NewUserRepository`），不是实例。框架对每个返回值执行 `i.Provide(bind)`，dig 会根据构造函数参数自动解析依赖并构造实例。

推荐顺序：先 Repository，再 Service，再 Handle，这样 Handle 依赖 Service、Service 依赖 Repository 的链可由 dig 自动推导。

示例（与 owl-admin 一致）：

```go
func (i *SubAppAdmin) Binds() []any {
    return []any{
        v1.NewUserHandle,
        v1.NewRoleHandle,
        // ...
        service.NewUserService,
        service.NewRoleService,
        // ...
        repository.NewUserRepository,
        repository.NewRoleRepository,
        // ...
    }
}
```

## 自定义 Provider 示例

可参考 `owl-admin/app/provider/jwt/jwt_service_provider.go`：

- 结构体含 `app foundation.Application`。
- `Register()` 里 `s.app.Register(func(c *conf.Configure) *JWTService { ... })`，从配置读取选项并返回 `*JWTService`。
- `Conf()` 返回 `map[string]string{"jwt.yaml": exampleConf}`，其中 `exampleConf` 可用 `//go:embed jwt.yaml` 嵌入。
- `Boot()` 可为空。

## 完成定义

- 能说明 ServiceProvider 四个方法在生命周期中的调用时机。
- 能说明 Binds() 中应放构造函数而非实例，以及推荐注册顺序。
- 能按 JWT Provider 模式写一个最小自定义 Provider。
