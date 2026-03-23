# 最小可运行子系统模板

本篇给出一套可以直接照着搭的新子系统骨架。目标不是覆盖全部能力，而是提供一个**最小可运行、可建表、可访问接口**的模板，适合作为 AI 与开发者的起点。

## 适用场景

- 新建一个独立仓库，基于 `owl` 启动 HTTP 服务。
- 不需要 RBAC、JWT、Redis 锁等复杂能力，先跑通一个公开 CRUD API。
- 后续再按需追加 `redis`、`permission`、`jwt`、`storage` 等 Provider。

## 最小目录结构

```text
my-order/
├── go.mod
├── main.go
└── app/
    ├── app.go
    ├── route/
    │   └── api.go
    ├── database/
    │   └── auto_migrate_gen.go
    ├── model/
    │   └── order.go
    ├── repository/
    │   └── order.go
    ├── service/
    │   └── order_service.go
    └── handle/
        └── v1/
            └── order_handle.go
```

## Provider 选择

对于这个最小模板，`owl` 已自动注册基础 Provider：

- `conf.ConfServiceProvider`
- `log.LogServiceProvider`
- `event.EventServiceProvider`
- `appconf.AppConfigServiceProvider`
- `validator.ValidatorServiceProvider`

你在子系统里最少只需要补：

| 场景 | 必选 Provider |
|------|---------------|
| 公开 HTTP API + 数据库 | `router.RouterServiceProvider`、`db.DBServiceProvider` |
| 再加分布式锁 | 上述 + `redis.RedisServiceProvider` |
| 再加后台权限 | 上述 + `permission.GuardProvider`、自定义 `jwt.JwtServiceProvider` |

## `go.mod`

```go
module bit-labs.cn/my-order

go 1.24.0

require bit-labs.cn/owl v0.0.0

replace bit-labs.cn/owl => ../owl
```

## `main.go`

```go
package main

import (
	"bit-labs.cn/owl"
	order "bit-labs.cn/my-order/app"
)

func main() {
	owl.NewApp(&order.SubAppOrder{}).WebShell()
}
```

## `app/app.go`

```go
package order

import (
	"bit-labs.cn/my-order/app/database"
	"bit-labs.cn/my-order/app/handle/v1"
	"bit-labs.cn/my-order/app/repository"
	"bit-labs.cn/my-order/app/route"
	"bit-labs.cn/my-order/app/service"
	"bit-labs.cn/owl"
	"bit-labs.cn/owl/contract/foundation"
	"bit-labs.cn/owl/provider/db"
	"bit-labs.cn/owl/provider/router"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type SubAppOrder struct {
	app foundation.Application
}

var _ owl.SubApp = (*SubAppOrder)(nil)

func (i *SubAppOrder) Name() string { return "order" }

func (i *SubAppOrder) ServiceProviders() []foundation.ServiceProvider {
	return []foundation.ServiceProvider{
		&router.RouterServiceProvider{},
		&db.DBServiceProvider{},
	}
}

func (i *SubAppOrder) Binds() []any {
	return []any{
		v1.NewOrderHandle,
		service.NewOrderService,
		repository.NewOrderRepository,
	}
}

func (i *SubAppOrder) RegisterRouters() {
	route.InitApi(i.app, i.Name())
}

func (i *SubAppOrder) Menu() []*router.Menu {
	return route.InitMenu()
}

func (i *SubAppOrder) Commands() []*cobra.Command { return nil }

func (i *SubAppOrder) Bootstrap() {
	i.app.Invoke(func(gdb *gorm.DB) {
		migDB := gdb.Session(&gorm.Session{Logger: gdb.Config.Logger.LogMode(logger.Error)})
		database.Migrate(migDB)
	})
}
```

## `app/route/api.go`

这个模板故意使用**公开接口**，避免你在第一次搭子系统时被鉴权链路卡住。

```go
package route

import (
	v1 "bit-labs.cn/my-order/app/handle/v1"
	"bit-labs.cn/owl"
	"bit-labs.cn/owl/contract/foundation"
	"bit-labs.cn/owl/provider/router"
	"github.com/gin-gonic/gin"
)

var orderMenu *router.Menu

func InitMenu() []*router.Menu {
	return []*router.Menu{orderMenu}
}

func InitApi(app foundation.Application, appName string) {
	err := app.Invoke(func(
		engine *gin.Engine,
		orderHandle *v1.OrderHandle,
	) {
		gv1 := engine.Group("/api/v1")

		r := router.NewRouteInfoBuilder(appName, orderHandle, gv1, router.MenuOption{
			ComponentName: "OrderList",
			Path:          "/order/index",
			Icon:          "ep:document",
		})

		r.Post("/orders", router.AccessPublic, orderHandle.Create).Name("创建订单").Build()
		r.Get("/orders", router.AccessPublic, orderHandle.Retrieve).Name("订单列表").Build()

		orderMenu = r.GetMenu()
	})
	owl.PanicIf(err)
}
```

## `app/model/order.go`

```go
package model

import (
	"bit-labs.cn/owl/provider/db"
)

const (
	OrderStatusPending = 1
	OrderStatusDone    = 2
)

type Order struct {
	db.BaseModel
	OrderNo string `gorm:"comment:订单号" json:"orderNo"`
	Title   string `gorm:"comment:标题" json:"title"`
	Status  int    `gorm:"comment:状态(1待处理,2已完成)" json:"status"`
}

func (Order) TableName() string {
	return "order_order"
}
```

## `app/repository/order.go`

```go
package repository

import (
	"context"

	"bit-labs.cn/my-order/app/model"
	"bit-labs.cn/owl/contract"
	"bit-labs.cn/owl/provider/db"
	"gorm.io/gorm"
)

type OrderRepositoryInterface interface {
	Save(order *model.Order) error
	Retrieve(page, pageSize int, fn func(db *gorm.DB)) (count int64, list []model.Order, err error)
	contract.WithContext[OrderRepositoryInterface]
}

type OrderRepository struct {
	db  *gorm.DB
	ctx context.Context
	db.BaseRepository[model.Order]
}

func NewOrderRepository(d *gorm.DB) OrderRepositoryInterface {
	return &OrderRepository{
		db:             d,
		BaseRepository: db.NewBaseRepository[model.Order](d),
	}
}

func (r *OrderRepository) WithContext(ctx context.Context) OrderRepositoryInterface {
	r.db = r.db.WithContext(ctx)
	r.ctx = ctx
	return r
}

func (r *OrderRepository) Save(order *model.Order) error {
	return r.BaseRepository.Save(order)
}

func (r *OrderRepository) Retrieve(page, pageSize int, fn func(db *gorm.DB)) (count int64, list []model.Order, err error) {
	return r.BaseRepository.Retrieve(page, pageSize, fn)
}
```

## `app/service/order_service.go`

```go
package service

import (
	"context"

	"bit-labs.cn/my-order/app/model"
	"bit-labs.cn/my-order/app/repository"
	"bit-labs.cn/owl/provider/db"
	"bit-labs.cn/owl/provider/router"
	validatorv10 "github.com/go-playground/validator/v10"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

type CreateOrderReq struct {
	OrderNo string `json:"orderNo" validate:"required,min=4,max=32" label:"订单号"`
	Title   string `json:"title" validate:"required,min=2,max=64" label:"标题"`
	Status  int    `json:"status" validate:"required,oneof=1 2" label:"状态"`
}

type RetrieveOrderReq struct {
	router.PageReq
	TitleLike string `json:"title" form:"title" validate:"omitempty,max=64" label:"标题"`
	Status    int    `json:"status" form:"status" validate:"omitempty,oneof=1 2" label:"状态"`
}

type OrderService struct {
	repo     repository.OrderRepositoryInterface
	validate *validatorv10.Validate
	_        *gorm.DB
}

func NewOrderService(
	repo repository.OrderRepositoryInterface,
	tx *gorm.DB,
	validate *validatorv10.Validate,
) *OrderService {
	return &OrderService{repo: repo, validate: validate, _: tx}
}

func (s *OrderService) CreateOrder(ctx context.Context, req *CreateOrderReq) error {
	if err := s.validate.Struct(req); err != nil {
		return err
	}
	var order model.Order
	if err := copier.Copy(&order, req); err != nil {
		return err
	}
	return s.repo.WithContext(ctx).Save(&order)
}

func (s *OrderService) RetrieveOrders(ctx context.Context, req *RetrieveOrderReq) (count int64, list []model.Order, err error) {
	if err := s.validate.Struct(req); err != nil {
		return 0, nil, err
	}
	return s.repo.WithContext(ctx).Retrieve(req.Page, req.PageSize, func(tx *gorm.DB) {
		db.AppendWhereFromStruct(tx, req)
		tx.Order("created_at desc")
	})
}
```

## `app/handle/v1/order_handle.go`

```go
package v1

import (
	"bit-labs.cn/my-order/app/service"
	"bit-labs.cn/owl/provider/router"
	"github.com/gin-gonic/gin"
)

type OrderHandle struct {
	svc *service.OrderService
}

func NewOrderHandle(svc *service.OrderService) *OrderHandle {
	return &OrderHandle{svc: svc}
}

func (h *OrderHandle) ModuleName() (string, string) { return "order", "订单管理" }

func (h *OrderHandle) Create(ctx *gin.Context) {
	var req service.CreateOrderReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		router.Fail(ctx, err)
		return
	}
	if err := h.svc.CreateOrder(ctx.Request.Context(), &req); err != nil {
		router.Fail(ctx, err)
		return
	}
	router.Success(ctx, nil)
}

func (h *OrderHandle) Retrieve(ctx *gin.Context) {
	var req service.RetrieveOrderReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		router.BadRequest(ctx, "参数绑定失败")
		return
	}
	count, list, err := h.svc.RetrieveOrders(ctx.Request.Context(), &req)
	if err != nil {
		router.Fail(ctx, err)
		return
	}
	router.PageSuccess(ctx, int(count), req.Page, req.PageSize, list)
}
```

## `app/database/auto_migrate_gen.go`

```go
package database

import (
	"bit-labs.cn/my-order/app/model"
	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) {
	_ = db.Migrator().AutoMigrate(
		&model.Order{},
	)
}
```

## 首次启动最小配置

### `conf/database.yaml`

建议第一次用 sqlite，最容易跑通：

```yaml
driver: sqlite
host: my-order.db
database: main
```

说明：

- `DBServiceProvider` 遇到 sqlite 时会把 `host` 拼到 `conf/` 目录下，所以最终数据库文件通常是 `conf/my-order.db`。
- 如果继续使用 pgsql/mysql，则按默认模板补齐连接信息即可。

### `conf/router.yaml`

默认模板通常已够用，确认端口即可：

```yaml
server:
  host: "0.0.0.0"
  port: 8080
```

## 这个模板跑通后再怎么扩展

1. 需要登录与权限：给路由分组挂权限中间件，并在 `ServiceProviders()` 中增加 `permission` 与自定义 `jwt` Provider。
2. 需要分布式锁：增加 `redis.RedisServiceProvider`，在 service 中注入 `redis.LockerFactory`。
3. 需要文件上传：增加 `storage.StorageServiceProvider`，并注册 `storage.NewFileHandle`。
4. 需要命令行：在 `Commands()` 返回 cobra 命令，或者用 `ConsoleShell()` 启动。

## 完成定义

- 按本文复制文件后，能在新仓库里启动一个最小公开 CRUD 子系统。
- 启动后至少能访问 `POST /api/v1/orders` 与 `GET /api/v1/orders`。
- 后续只需在此基础上叠加鉴权、缓存、上传等能力，而不是重新猜项目骨架。
