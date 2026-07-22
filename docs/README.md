# Owl 框架文档

本目录为 **Owl** 应用框架的开发者手册，面向 AI 开发工具与人工开发者。阅读后可基于框架创建新的子系统（独立项目）或理解现有子应用的装配方式。

## 阅读顺序

建议按以下顺序阅读，从理解生命周期到落地新项目：


| 顺序  | 文档                                                                   | 说明                     |
| --- | -------------------------------------------------------------------- | ---------------------- |
| 1   | [01-application-lifecycle.md](01-application-lifecycle.md)           | 应用入口与启动生命周期            |
| 2   | [02-subapp-contract.md](02-subapp-contract.md)                       | SubApp 契约与 `app` 字段注入  |
| 3   | [03-provider-and-di.md](03-provider-and-di.md)                       | Service Provider 与依赖注入 |
| 4   | [04-routing-config-runtime.md](04-routing-config-runtime.md)         | 路由、菜单、配置与运行目录          |
| 5   | [05-create-new-subapp-playbook.md](05-create-new-subapp-playbook.md) | 从 0 到 1 创建新子系统         |
| 6   | [06-pitfalls-checklist.md](06-pitfalls-checklist.md)                 | 常见坑与自检清单               |
| 7   | [07-minimal-subapp-template.md](07-minimal-subapp-template.md)       | 可直接复制的最小子系统样板          |
| 8   | [08-startup-and-verification.md](08-startup-and-verification.md)     | 启动、建表、路由与接口验证闭环        |


## 术语表


| 术语                    | 含义                                                          |
| --------------------- | ----------------------------------------------------------- |
| **SubApp**            | 子应用，实现 `owl.SubApp` 接口的模块，可挂到 `owl.NewApp(subApps...)` 上    |
| **Service Provider**  | 服务提供者，实现 `foundation.ServiceProvider`，负责 Register/Boot/Conf |
| **Binds**             | SubApp 返回的构造函数列表，用于向 DI 容器注册 Repository/Service/Handle      |
| **WebShell**          | HTTP 模式启动，会执行 RegisterRouters、RegisterMenus、Bootstrap，最后启动 Gin |
| **ConsoleShell**      | 命令行模式启动，只执行 Bootstrap，并将 `RegisterCommands()` 挂到 Cobra           |
| **basePath / runDir** | 框架推断路径时优先用可执行文件所在目录（basePath），不存在时用当前工作目录（runDir）           |


## 相关项目与文档索引

- **owl-admin**：基于 Owl 的后台管理子应用（RBAC、用户/角色/菜单等）。  
**若要在现有后台中新增业务模块**，请阅读 owl-admin 的 `docs/`：架构总览 → 标准模块模板（position）→ 路由/菜单/权限 → 迁移/Seeder/事件 → 新增模块操作清单 → 进阶与常见坑。  
同组织下 owl-admin 仓库路径：`owl-admin/docs/`。
- **owl-ui**：前端框架包（Vue3 + 路由/权限/子系统）。新建前端子系统或理解宿主/子系统边界时见 `owl-ui/docs/`。
- **owl-admin-ui**：后台前端子系统包。在后台里新增业务页面时见 `owl-admin-ui/docs/`。

## 推荐工作流

如果你的目标是“今天就把一个新子系统跑起来”，建议直接按这个顺序：

1. 先看 [05-create-new-subapp-playbook.md](05-create-new-subapp-playbook.md) 了解整体步骤。
2. 再按 [07-minimal-subapp-template.md](07-minimal-subapp-template.md) 复制最小可运行样板。
3. 最后按 [08-startup-and-verification.md](08-startup-and-verification.md) 做启动与 smoke test。

