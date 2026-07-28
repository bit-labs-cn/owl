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
6. 注册基础 Service Provider：`registerBaseServiceProviders()`（conf、log、event、appconf、validator）——先 `generateProviderConfigs` 落盘默认配置，再 `registerProviders` 注册 DI 工厂；**此时不** `Invoke` Logger，避免过早构造 `Configure`
7. 保存传入的 `SubApp` 列表

此后调用 `WebShell()` 或 `ConsoleShell()` 时，才会进入 `newSubApp()` 完成子应用与 Provider 的装配。

## 配置生成与加载时机

为避免首次启动时「配置尚未生成就被 `NewConfigure` Walk」导致 `GetConfig` panic，框架采用两阶段：

1. **先 Conf 落盘**：各 Provider 的 `Conf()`（embed 默认 yaml）在文件不存在时写入 `conf/`；已存在则 `ValidateConfigKeys` 告警缺键，不覆盖。
2. **再 Register**：只向 dig 注册构造函数，不读配置。
3. **全部（base + 子应用）Conf 写完后**：`initLogger()` → `Invoke(Logger)` 首次触发 `NewConfigure`，一次性加载 `conf/` 下全部文件。

顺序概览：`base: generateConf → Register` → 收集子应用 Provider → `sub: generateConf → Register` → `initLogger`（加载 Configure）→ `RegisterRouters` / `Bootstrap` / `Boot` …

## newSubApp() 装配顺序

`newSubApp(subApps...)` 是子应用与基础设施的装配中心，顺序如下：

1. **注入 app 并注册 Binds**  
   对每个 SubApp：`injectAppInstance(app)`，再将 `app.Binds()` 中的每个构造函数 `Provide` 进容器。

2. **收集命令**  
   `i.commands = append(i.commands, app.RegisterCommands()...)`

3. **收集 Service Provider**  
   `i.serviceProviders = append(i.serviceProviders, app.ServiceProviders()...)`

4. **先生成配置，再注册 Provider**  
   `generateProviderConfigs(i.serviceProviders...)` 落盘子应用缺失的配置文件；`registerProviders(i.serviceProviders...)` 只调用 `Register()`。

5. **首次加载 Configure**  
   `initLogger()`：解析 Logger 并触发 `NewConfigure` 加载 conf（此时 base 与子应用配置均已落盘）。

6. **按模式执行子应用**  
   - **非命令行**：先对每个 SubApp 依次 `RegisterRouters()`、收集 `RegisterMenus()`，再 `AddMenu(i.menus...)`，最后对每个 SubApp 执行 `Bootstrap()`。
   - **命令行**：对每个 SubApp 只执行 `Bootstrap()`。  
   `Bootstrap()` 负责初始化配置/数据等；表结构迁移由接口方法声明，不要在 Bootstrap 里直接 `AutoMigrate`。

7. **统一数据库迁移（hash 门控）**  
   全部 `Bootstrap()` 结束后，框架执行：各 SubApp 的 `BeforeMigrate`（**串行**）→ 对每个 `RegisterMigrate()` 返回的 Model **分别**计算 schema hash（**并行**；写入 `storage/migrate_hash.txt`，每行 `类型名=hash`）→ **仅对变化的 Model** 串行执行 `AutoMigrate` → 各 SubApp 的 `AfterMigrate`（**并行**，全部完成或任一失败后再继续）。未变化的 Model 跳过，Before/After 始终执行。  
   `RegisterRouters`、`Bootstrap`、`Provider.Boot` 仍串行。幂等种子若可接受延迟，可在 `AfterMigrate` 内自行 `go`（如 owl-admin 字典种子），框架层不会 fire-and-forget。

8. **执行所有 Provider 的 Boot()**  
   `for _, serviceProvider := range i.serviceProviders { serviceProvider.Boot() }`

9. **Web 模式**  
   `WebShell()` 最后 `Invoke` 解析 `*router.RouterServiceProvider` 并调用 `Run()` 启动 HTTP。

10. **Console 模式**  
    `ConsoleShell()` 将 `i.commands` 挂到 `rootCmd` 并执行 `rootCmd.Execute()`。

## 路径推断规则

配置、存储、语言包等目录通过 `GetConfigPath()`、`GetStoragePath()` 等获取，内部使用 `inferDir(path)`：

- 优先：`filepath.Join(basePath, path)`（可执行文件所在目录）
- 若该路径不存在：`filepath.Join(runDir, path)`（当前工作目录）

因此开发时与打成二进制后，若运行目录不同，实际使用的 `conf/`、`storage/` 可能不同，部署时需注意。

## 完成定义

- 能说明 `NewApp` → `registerBase*` → `WebShell/ConsoleShell` → `newSubApp` 的调用顺序。
- 能说明「先 Conf 落盘 → 再 Register → 再 initLogger/NewConfigure」为何能避免首次启动缺配置 panic。
- 能说明 Web 与 Console 模式下 SubApp 的 `RegisterRouters`、`RegisterMenus`、`Bootstrap` 是否执行。
- 能说明 `basePath` 与 `runDir` 对配置与存储目录的影响。
