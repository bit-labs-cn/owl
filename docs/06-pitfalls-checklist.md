# 常见坑与自检清单

## 隐式约束

1. **SubApp 与 ServiceProvider 必须有 `app` 字段**  
   框架通过 `reflect.FieldByName("app")` 注入，字段名必须为 `app`，类型建议 `foundation.Application`，否则运行时报错。

2. **Binds() 返回的是构造函数，不是实例**  
   返回 `NewXxxHandle` 而不是 `NewXxxHandle(...)` 的调用结果；dig 会根据参数自动解析依赖。

3. **漏注册 Binds**  
   新增了 Handle/Service/Repository 但未在 SubApp 的 `Binds()` 中 `Provide`，会导致 `Invoke` 时 dig 无法解析依赖，启动失败。

## 配置与路径

4. **配置目录与运行目录**  
   `GetConfigPath()` 等先使用可执行文件所在目录（basePath），不存在再用当前工作目录（runDir）。打包成 systemd 服务或 Docker 时，需确认工作目录与预期一致，否则 `conf/`、`storage/` 可能指向错误位置。

5. **环境变量覆盖规则**  
   见 `provider/conf/ENV_VARIABLES.md`，键名与前缀需正确，否则配置可能未按预期覆盖。

## 路由与权限

6. **权限依据运行时路由表**  
   权限校验依赖 `router.GetAllRoutes()` 等运行时注册的路由元数据，而不是数据库中的“接口表”。若文档或旧代码中提及“接口表”，不要误以为改表即可改权限。

7. **Swagger 与真实路由**  
   若使用 Swagger 注解，需保证 `@Router` 等与真实注册路径一致，否则文档与行为不一致。

## 版本与依赖

8. **owl 与子应用版本**  
   子应用通过 `replace` 引用本地 owl 时，需确保两边兼容；升级 owl 后建议跑一遍子应用启动与核心接口。

9. **foundation.Application 未实现方法**  
   部分 `foundation.Application` 方法在当前实现中为 `panic("implement me")`，文档与代码不要依赖这些方法，除非已确认实现。

## 自检清单（新子系统上线前）

- [ ] SubApp 结构体有 `app foundation.Application` 字段。
- [ ] 所有新增的 Handle、Service、Repository 构造函数已加入 `Binds()`。
- [ ] 路由在 `RegisterRouters()` 中通过 `NewRouteInfoBuilder` 正确注册，且访问级别与权限中间件一致。
- [ ] 新 model 已加入 `database.Migrate()` 的 `AutoMigrate` 列表，且 `Bootstrap()` 中调用了迁移。
- [ ] 若使用自定义 Provider，其结构体也有 `app` 字段，且已在 `ServiceProviders()` 中注册。
- [ ] 部署环境下 `conf/`、`storage/` 路径符合预期（basePath/runDir）。
- [ ] 敏感配置通过环境变量或外部配置管理，未提交到仓库。

## 完成定义

- 能列出至少 5 条常见坑并说明原因。
- 能按自检清单检查一个新子系统的接线与配置是否完整。
