# Owl Framework 🦉

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Owl 是一个模块化、功能完备的 Go 语言企业级开发框架。它采用依赖注入（Dependency Injection）设计模式，集成了现代 Web 开发所需的丰富组件，旨在帮助开发者快速构建高性能、可扩展的后端应用。

## ✨ 核心特性

Owl 框架采用 "Battery-Included" 理念，开箱即用以下功能：

*   **核心架构**
    *   基于 **Service Provider** 的模块化设计
    *   强大的依赖注入容器 (Container)
    *   统一的配置管理 (YAML/Env)

*   **Web & API**
    *   集成 **Gin** Web 框架，提供高性能路由处理
    *   内置中间件：Recovery, Logger, CORS, RateLimit, Security, RequestID
    *   支持 Socket.IO 实时通信

*   **数据存储**
    *   集成 **GORM**，支持 MySQL, PostgreSQL, SQLite
    *   **Redis** 缓存与分布式锁支持
    *   多云对象存储抽象层 (Storage)：
        *   本地存储 (Local)
        *   AWS S3 / MinIO
        *   阿里云 OSS
        *   腾讯云 COS
        *   七牛云 Kodo

*   **消息队列 & 事件**
    *   内置 **MQTT** 客户端支持
    *   集成 **RabbitMQ**
    *   应用内事件总线 (EventBus)

*   **企业级能力**
    *   **支付聚合**：支持微信支付、支付宝、银行卡支付
    *   **OCR 服务**：集成阿里云、百度云、腾讯云 OCR
    *   **权限管理**：集成 Casbin 权限控制
    *   **国际化 (i18n)**：支持多语言响应
    *   **日志系统**：结构化日志记录与轮转

## 🛠️ 快速开始

### 环境要求

*   Go 1.20+

### 安装

```bash
git clone https://github.com/your-org/owl.git
cd owl
go mod download
```

### 运行

```bash
go run main.go
```

## 📂 项目结构

```text
owl/
├── contract/       # 接口定义 (Contracts)
├── provider/       # 服务提供者实现 (Providers)
│   ├── appconf/    # 应用配置
│   ├── db/         # 数据库连接 (GORM)
│   ├── router/     # HTTP 路由 (Gin)
│   ├── redis/      # Redis 缓存
│   ├── storage/    # 对象存储 (S3, OSS, etc.)
│   ├── pay/        # 支付网关
│   ├── ocr/        # OCR 服务
│   └── ...
├── utils/          # 通用工具库
├── lang/           # 国际化语言包
├── go.mod          # 依赖管理
└── application.go  # 应用入口
```

## 📚 开发文档

基于本框架**创建新子系统（独立项目）**或理解 SubApp、Provider、路由与配置的详细说明见 **[docs/](docs/)** 目录，包含应用生命周期、SubApp 契约、Provider 与 DI、路由与配置、创建新子系统的标准步骤及常见坑与自检清单。适合开发人员与 AI 开发工具按顺序阅读后直接开发新项目。

## 📝 许可证

本项目采用 [MIT License](LICENSE) 许可证。
