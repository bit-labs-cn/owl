# 应用入口与生命周期

## 入口形式

框架通过 `owl.NewApp(subApps...)` 创建应用，再根据运行方式选择：

- **WebShell()**：HTTP 服务，注册路由与菜单，最后启动 Gin。
- **ConsoleShell(rootCmd)**：命令行模式，只执行子应用 `Bootstrap()` 并将 `RegisterCommands()` 挂到 Cobra。

代码入口：`application.go` 中的 `NewApp`、`WebShell`、`ConsoleShell`、`newSubApp`。

### 典型宿主入口

**Web 单子应用（如 owl-admin）：**

```go
// main.go
func main() {
    var subApps = []owl.SubApp{ &admin.SubAppAdmin{} }
    owl.NewApp(subApps...).WebShell()
}
```

**仅命令行（如部分服务项目）：**

```go
owl.NewApp().ConsoleShell(rootCmd)
```

## NewApp() 做了什么

在 `application.go` 的 `NewApp()` 中依次：

1. 初始化雪花 ID：`utils.InitSnowFlakeWorker(1, 3)`
2. 创建 DI 容器：`dig.New()`
3. 设置路径：`setPath()`（得到 `runDir`、`basePath`）
4. 确保配置目录存在：`ensureConfDir()`
5. 注册基础绑定：`registerBaseBindings()`（Application、MenuRepository）
6. 注册基础 Service Provider：`registerBaseServiceProviders()`（conf、log、event、appconf、validator）
7. 保存传入的 `SubApp` 列表

此后调用 `WebShell()` 或 `ConsoleShell()` 时，才会进入 `newSubApp()` 完成子应用与 Provider 的装配。

## newSubApp() 装配顺序

`newSubApp(subApps...)` 是子应用与基础设施的装配中心，顺序如下：

1. **注入 app 并注册 Binds**  
   对每个 SubApp：`injectAppInstance(app)`，再将 `app.Binds()` 中的每个构造函数 `Provide` 进容器。

2. **收集命令**  
   `i.commands = append(i.commands, app.RegisterCommands()...)`

3. **收集 Service Provider**  
   `i.serviceProviders = append(i.serviceProviders, app.ServiceProviders()...)`

4. **统一注册所有 Provider**  
   `registerServiceProviders(i.serviceProviders...)`：对每个 Provider 执行 `injectAppInstance`、`Register()`，并根据 `Conf()` 生成缺失的配置文件。

5. **按模式执行子应用**  
   - **非命令行**：先对每个 SubApp 依次 `RegisterRouters()`、收集 `RegisterMenus()`，再 `AddMenu(i.menus...)`，最后对每个 SubApp 执行 `Bootstrap()`。
   - **命令行**：对每个 SubApp 只执行 `Bootstrap()`。  
   `Bootstrap()` 负责初始化配置/数据等；表结构迁移由接口方法声明，不要在 Bootstrap 里直接 `AutoMigrate`。

6. **统一数据库迁移（hash 门控）**  
   全部 `Bootstrap()` 结束后，框架依次调用各 SubApp 的 `BeforeMigrate` → 对每个 `RegisterMigrate()` 返回的 Model **分别**计算 schema hash（写入 `storage/migrate_hash.txt`，每行 `类型名=hash`）→ **仅对变化的 Model** 执行 `AutoMigrate` → 各 SubApp 的 `AfterMigrate`。未变化的 Model 跳过，Before/After 始终执行。

7. **执行所有 Provider 的 Boot()**  
   `for _, serviceProvider := range i.serviceProviders { serviceProvider.Boot() }`

8. **Web 模式**  
   `WebShell()` 最后 `Invoke` 解析 `*router.RouterServiceProvider` 并调用 `Run()` 启动 HTTP。

9. **Console 模式**  
   `ConsoleShell()` 将 `i.commands` 挂到 `rootCmd` 并执行 `rootCmd.Execute()`。

## 路径推断规则

配置、存储、语言包等目录通过 `GetConfigPath()`、`GetStoragePath()` 等获取，内部使用 `inferDir(path)`：

- 优先：`filepath.Join(basePath, path)`（可执行文件所在目录）
- 若该路径不存在：`filepath.Join(runDir, path)`（当前工作目录）

因此开发时与打成二进制后，若运行目录不同，实际使用的 `conf/`、`storage/` 可能不同，部署时需注意。

## 完成定义

- 能说明 `NewApp` → `registerBase*` → `WebShell/ConsoleShell` → `newSubApp` 的调用顺序。
- 能说明 Web 与 Console 模式下 SubApp 的 `RegisterRouters`、`RegisterMenus`、`Bootstrap` 是否执行。
- 能说明 `basePath` 与 `runDir` 对配置与存储目录的影响。
