<!-- markdownlint-disable MD010 -->

# user 完整链路示例模板（owl-admin）

按 `owl-admin` 里的 `user` 模块真实链路组织模板。  
目标是让你在新增后台模块时，能直接照着 `user` 这条链路落地：`app -> route -> model -> repository -> service -> handle -> event/listener -> migrate`。

## 适用场景

- 你要在 `owl-admin` 里新增模块，希望直接照着现有 `user` 模块的业务接线方式写代码。
- 你希望参考一个已上线风格的“完整链路”，而不是“独立子系统最小起步”。
- 你的子系统会依赖 `owl` 和 `admin`，登录态与权限体系由上层承接，不需要在子系统里自己实现鉴权中间件。
- 你接受示例包含 `redis locker`、`event bus`、菜单/角色协作这些真实复杂度。

## 示例文件清单（对应 user 链路）

```text
owl-admin/
└── app/
    ├── app.go
    ├── route/
    │   └── api.go
    ├── model/
    │   └── user.go
    ├── repository/
    │   └── user.go
    ├── service/
    │   └── user_service.go
    ├── handle/
    │   └── v1/
    │       └── user_handle.go
    ├── event/
    │   └── assign_role_to_user.go
    ├── listener/
    │   └── listener.go
    └── database/
        └── auto_migrate_gen.go
```

## `app/app.go`（Provider + Binds + Bootstrap）

`user` 链路所在的 `SubAppAdmin` 是完整后台应用，真实注册的 provider 如下。  
如果你的子系统依赖 `owl` 和 `admin`，这里更重要的是理解 `Binds()`、`Bootstrap()` 和业务接线方式，而不是把权限 provider 或鉴权实现整段照搬到自己的子系统里：

```go
func (i *SubAppAdmin) ServiceProviders() []foundation.ServiceProvider {
	return []foundation.ServiceProvider{
		&permission.GuardProvider{},
		&router.RouterServiceProvider{},
		&db.DBServiceProvider{},
		&jwt.JwtServiceProvider{},
		&redis.RedisServiceProvider{},
		&socketio.SocketIOServiceProvider{},
		&captcha.CaptchaServiceProvider{},
		&storage.StorageServiceProvider{},
	}
}

func (i *SubAppAdmin) Binds() []any {
	return []any{
		// ... 省略其他模块
		v1.NewUserHandle,
		service.NewUserService,
		repository.NewUserRepository,
	}
}

func (i *SubAppAdmin) Bootstrap() {
	i.app.Invoke(func(gdb *gorm.DB) {
		migDB := gdb.Session(&gorm.Session{Logger: gdb.Config.Logger.LogMode(logger.Error)})
		go database.Migrate(migDB)
		go seeder.InitAllDictData(migDB)
		listener.Init(i.app)
	})
}
```

## `app/route/api.go`（user 路由注册）

这里直接关注 `user` 的业务路由定义。  
权限链路由 `admin` 承接，你的子系统不需要自己再实现 `PermissionCheck`；通常只需要像 `user` 一样声明路由与访问级别：

```go
gv1 := engine.Group("/api/v1")
gv1.Use(middleware2.OperationLog(logService))

// user
{
	r := router.NewRouteInfoBuilder(appName, userHandle, gv1, router.MenuOption{
		ComponentName: "SystemUser",
		Path:          "/system/user/index",
		Icon:          "ep:user",
	})

	r.Use(middleware.RateLimiter(time.Second*1, 2)).
		Post("/users/login", router.AccessPublic, userHandle.Login).
		Name("用户登录").WithoutOperateLog().Build()

	r.Put("/users/me/password", router.AccessAuthenticated, userHandle.ChangePassword).Name("修改我的密码").Build()
	r.Get("/users/me/menus", router.AccessAuthenticated, userHandle.GetMyMenus).Name("我的菜单").Build()
	r.Get("/users/me/permissions", router.AccessAuthenticated, userHandle.GetMyPermissions).Name("我的权限").Build()
	r.Get("/users/me", router.AccessAuthenticated, userHandle.Me).Name("我的信息").Build()

	r.Post("/users", router.AccessAuthorized, userHandle.Create).Name("创建用户").Build()
	r.Delete("/users/:id", router.AccessAuthorized, userHandle.Delete).Name("删除用户").Build()
	r.Put("/users/:id", router.AccessAuthorized, userHandle.Update).Name("更新用户").Build()
	r.Put("/users/:id/status", router.AccessAuthorized, userHandle.ChangeStatus).Name("启用，禁用用户").Build()
	r.Get("/users", router.AccessAuthorized, userHandle.Retrieve).Name("分页获取用户").Build()
	r.Get("/users/:id", router.AccessAuthorized, userHandle.Detail).Name("获取用户详情").Build()
	r.Put("/users/:id/reset", router.AccessSuperAdmin, userHandle.ResetPassword).Name("重置用户密码").Build()
	r.Put("/users/:id/avatar", router.AccessAuthorized, userHandle.ChangeAvatar).Name("修改用户头像").Build()

	r.Post("/users/:id/roles", router.AccessAuthorized, userHandle.AssignRolesToUser).Name("分配角色给用户").Build()
	r.Get("/users/:id/roles", router.AccessAuthorized, userHandle.GetRoleIdsByUserId).Name("获取用户角色").Build()

	userMenu = r.GetMenu()
}
```

## `app/model/user.go`（用户聚合模型）

包含用户主表 + 角色/菜单多对多 + 超管构造：

```go
type User struct {
	db.BaseModel
	Avatar   string `gorm:"comment:用户头像" json:"avatar"`
	Username string `gorm:"comment:用户名称;type:string;size:512" json:"username"`
	Nickname string `gorm:"comment:用户昵称;type:string;size:128" json:"nickname"`
	Password string `gorm:"comment:用户密码" json:"-"`
	Remark   string `gorm:"comment:remark" json:"remark"`
	Phone    string `gorm:"comment:手机;type:string;size:32" json:"phone"`
	Email    string `gorm:"comment:邮箱" json:"email"`
	Status   int    `gorm:"comment:状态" json:"status"`
	Sex      int    `gorm:"comment:性别" json:"sex"`
	Source   string `gorm:"comment:用户来源" json:"source"`
	SourceID string `gorm:"comment:第三方用户唯一标识" json:"sourceID"`

	Roles []Role `gorm:"many2many:admin_user_role;joinForeignKey:user_id;References:id;JoinReferences:role_id" json:"roles"`
	Menus []Menu `gorm:"many2many:admin_user_menu;joinForeignKey:user_id;References:id;JoinReferences:menu_id" json:"menus"`

	Permissions  []string `json:"permissions" gorm:"-"`
	IsSuperAdmin bool     `json:"isSuperAdmin" gorm:"-"`
}

func (i *User) TableName() string { return "admin_user" }

func (i *User) SetPassword(newPassword string) {
	i.Password = utils.BcryptHash(newPassword)
}

func NewSuperUser() User {
	return User{
		BaseModel:    db.BaseModel{ID: 19941996},
		Username:     "glen",
		Nickname:     "超级管理员",
		IsSuperAdmin: true,
		Permissions:  []string{"*:*:*"},
		Roles:        []Role{{Name: "superAdmin"}},
	}
}
```

## `app/repository/user.go`（数据持久化）

`user` 仓储除常规查询外，还有 `(username, source)` 唯一性校验和角色关联替换。  
如果你要补齐一个资源的**增删改查**线路，仓储层至少要提供下面这些接口：

```go
type UserRepositoryInterface interface {
	FindById(id any) (*model.User, error)
	Unique(id uint, username string, source string) bool
	Save(user *model.User) error
	Delete(ids ...any) error
	Retrieve(page, pageSize int, fn func(db *gorm.DB)) (count int64, list []model.User, err error)
	contract.WithContext[UserRepositoryInterface]
}

func (i *UserRepository) WithContext(ctx context.Context) UserRepositoryInterface {
	i.db = i.db.WithContext(ctx)
	i.ctx = ctx
	return i
}

func (i *UserRepository) Save(user *model.User) error {
	err := i.db.Save(&user).Error
	if err != nil {
		return err
	}
	err = i.db.Model(&user).Association("Roles").Replace(&user.Roles)
	return err
}

func (i *UserRepository) Unique(id uint, username string, source string) bool {
	_, exists := i.BaseRepository.Unique(id, func(db *gorm.DB) {
		db.Where("username", username).Where("source", source)
	})
	return exists
}

func (i *UserRepository) FindById(id any) (*model.User, error) {
	var user model.User
	err := i.db.Where("id = ?", id).Preload("Roles").First(&user).Error
	return &user, err
}

func (i *UserRepository) Retrieve(page, pageSize int, fn func(db *gorm.DB)) (count int64, list []model.User, err error) {
	return i.BaseRepository.Retrieve(page, pageSize, fn)
}
```

删除场景直接复用 `BaseRepository.Delete(id)`，通常不需要每个仓储再单独重写一份。

## `app/service/user_service.go`（业务编排）

`user` service 是整条链路最核心的编排层：参数校验、锁、复制、鉴权协作、事件发布。

### 业务错误约定

对“已存在 / 不存在 / 状态不允许 / 重复操作”这类**领域错误**，推荐在 service 包内统一封装业务错误，不要在业务逻辑里直接散落裸 `errors.New(...)`。

示例：

```go
const (
	CodeUserExists   = "USER_EXISTS"
	CodeUserNotFound = "USER_NOT_FOUND"
)

func UserExists() *errContract.BizError {
	return errContract.NewBizError(CodeUserExists, "用户已存在")
}

func UserNotFound() *errContract.BizError {
	return errContract.NewBizError(CodeUserNotFound, "用户不存在")
}
```

后续 `Create/Update/AssignRoleToUser` 这类写操作里，遇到业务分支应优先返回上述业务错误；数据库、网络、序列化等底层异常再直接返回原始 `error`。

### 关键结构与登录

```go
type UserService struct {
	db         *gorm.DB
	menuManger *router.MenuRepository
	jwtSvc     *jwt.JWTService
	db.BaseRepository[model.User]
	roleSvc   *RoleService
	enforcer  casbin.IEnforcer
	userRepo  repository.UserRepositoryInterface
	eventBus  EventBus.Bus
	configure *conf.Configure
	locker    redis.LockerFactory
	validate  *validatorv10.Validate
}

func (i *UserService) Login(ctx context.Context, req *LoginReq) (resp *LoginResp, err error) {
	if err := i.validate.Struct(req); err != nil {
		return nil, err
	}
	user, err := i.GetUserByName(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if ok := utils.BcryptCheck(req.Password, user.Password); !ok {
		return nil, ErrLogin
	}
	token, err := i.jwtSvc.GenerateToken(user)
	return &LoginResp{User: user, AccessToken: token}, err
}
```

### 增删改查（锁 + 唯一性 + copier）

```go
func (i *UserService) RetrieveUsers(ctx context.Context, req *RetrieveUserReq) (count int, list []model.User, err error) {
	if err = i.validate.Struct(req); err != nil {
		return 0, nil, err
	}

	c, u, e := i.userRepo.WithContext(ctx).Retrieve(req.Page, req.PageSize, func(tx *gorm.DB) {
		db.AppendWhereFromStruct(tx, req)
		tx.Preload("Roles")
		tx.Order("created_at desc")
	})
	return cast.ToInt(c), u, e
}

func (i *UserService) CreateUser(ctx context.Context, req *CreateUserReq) error {
	if err := i.validate.Struct(req); err != nil {
		return err
	}
	l := i.locker.New()
	if err := l.Lock("user:create"); err != nil {
		return err
	}
	defer l.Unlock()

	if i.userRepo.WithContext(ctx).Unique(0, req.Username, req.Source) {
		return UserExists()
	}

	var user model.User
	if err := copier.Copy(&user, req); err != nil {
		return err
	}
	user.SetPassword(req.Password)
	return i.userRepo.WithContext(ctx).Save(&user)
}

func (i *UserService) UpdateUser(ctx context.Context, req *UpdateUserReq) error {
	if err := i.validate.Struct(req); err != nil {
		return err
	}
	l := i.locker.New()
	if err := l.Lock("user:update:" + cast.ToString(req.ID)); err != nil {
		return err
	}
	defer l.Unlock()

	if i.userRepo.WithContext(ctx).Unique(req.ID, req.Username, req.Source) {
		return UserExists()
	}
	user, err := i.userRepo.WithContext(ctx).FindById(req.ID)
	if err != nil {
		return err
	}
	if err = copier.Copy(&user, req); err != nil {
		return err
	}
	return i.userRepo.WithContext(ctx).Save(user)
}

func (i *UserService) DeleteUser(ctx context.Context, id uint) error {
	l := i.locker.New()
	if err := l.Lock("user:delete:" + cast.ToString(id)); err != nil {
		return err
	}
	defer l.Unlock()

	return i.BaseRepository.Delete(id)
}
```

如果你的模块需要详情接口，通常做法也是在 service 里先调 `repo.WithContext(ctx).FindById(id)`，必要时把 `gorm.ErrRecordNotFound` 转成业务错误，再返回给 handle。

### 分配角色（发布事件同步 casbin 分组策略）

```go
func (i *UserService) AssignRoleToUser(ctx context.Context, req *AssignRoleToUser) error {
	if err := i.validate.Struct(req); err != nil {
		return err
	}
	l := i.locker.New()
	if err := l.Lock("user:assign-roles:" + cast.ToString(req.UserID)); err != nil {
		return err
	}
	defer l.Unlock()

	roles := db.GetModelsByIDs[model.Role](req.RoleIDs)
	user, err := i.userRepo.WithContext(ctx).FindById(req.UserID)
	if err != nil {
		return err
	}
	user.SetRoles(roles)
	err = i.userRepo.WithContext(ctx).Save(user)
	i.eventBus.Publish(event.AssignRoleToUser, req)
	return err
}
```

## `app/handle/v1/user_handle.go`（HTTP 入口）

handle 只做绑定参数、调 service、返回统一响应。下面这组就是 `user` 模块里最典型的 CRUD 入口：

```go
func (i *UserHandle) Create(ctx *gin.Context) {
	req := new(service.CreateUserReq)
	if err := ctx.ShouldBindJSON(&req); err != nil {
		router.Fail(ctx, err)
		return
	}
	if err := i.userSvc.CreateUser(ctx.Request.Context(), req); err != nil {
		router.Fail(ctx, err)
		return
	}
	router.Success(ctx, nil)
}

func (i *UserHandle) Delete(ctx *gin.Context) {
	id := cast.ToUint(ctx.Param("id"))
	err := i.userSvc.DeleteUser(ctx.Request.Context(), id)
	if err != nil {
		router.Fail(ctx, err)
		return
	}
	router.Success(ctx, nil)
}

func (i *UserHandle) Update(ctx *gin.Context) {
	req := new(service.UpdateUserReq)
	if err := ctx.ShouldBindJSON(&req); err != nil {
		router.Fail(ctx, err)
		return
	}
	req.ID = cast.ToUint(ctx.Param("id"))
	err := i.userSvc.UpdateUser(ctx.Request.Context(), req)
	if err != nil {
		router.Fail(ctx, err)
		return
	}
	router.Success(ctx, nil)
}

func (i *UserHandle) Retrieve(ctx *gin.Context) {
	var req service.RetrieveUserReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		router.BadRequest(ctx, "参数绑定失败")
		return
	}
	count, list, err := i.userSvc.RetrieveUsers(ctx.Request.Context(), &req)
	if err != nil {
		router.Fail(ctx, err)
		return
	}
	router.PageSuccess(ctx, count, req.Page, req.PageSize, list)
}
```

`user` 当前示例里 `Detail()` 还是空实现，因此本文重点展示可直接复用的 Create / Delete / Update / Retrieve 四条主链路。

## `app/event/assign_role_to_user.go` + `app/listener/listener.go`

即使你的业务子系统不自己实现权限中间件，`user` 这段事件链路依然值得参考：它展示了“service 发布事件，listener 做后置同步”的写法。  
在 `admin` 本体里，这个事件用于把“用户-角色关系”同步到 casbin grouping policy：

```go
const (
	AssignRoleToUser = "assign_role_to_user"
)
```

```go
bus.Subscribe(event.AssignRoleToUser, func(req *service.AssignRoleToUser) {
	userID := cast.ToString(req.UserID)
	var rules [][]string
	for _, roleID := range req.RoleIDs {
		rules = append(rules, []string{userID, roleID})
	}
	_, err := enforcer.RemoveFilteredGroupingPolicy(0, userID)
	log.Error(err)
	_, err = enforcer.AddGroupingPolicies(rules)
	log.Error(err)
})
```

## `app/database/auto_migrate_gen.go`（迁移）

确保 `User` 与 `UserMenu` 已在迁移列表中：

```go
func Migrate(db *gorm.DB) {
	_ = db.Migrator().AutoMigrate(
		// ... 省略其他模型
		&User{},
		&UserMenu{},
	)
}
```

## 启动与验证（owl-admin 场景）

本示例基于现有 `owl-admin` 工程，不需要独立 `go.mod/main.go`。  
启动与验收请按以下文档执行：

- `owl-admin/docs/08-startup-and-verification.md`
- `owl-admin/docs/05-create-new-module-playbook.md`

建议最少验收以下接口：

- `POST /api/v1/users/login`
- `GET /api/v1/users/me`
- `GET /api/v1/users/me/menus`
- `GET /api/v1/users`
- `POST /api/v1/users/:id/roles`

## 完成定义

- 文档中的示例代码已从 `order` 最小公开模板切换为 `owl-admin user` 完整链路。
- 文档读者可以按“层次 + 接线点”理解 `user` 模块，而不是只看到最小 CRUD。
- 文档已去掉“子系统需要自己实现鉴权中间件”的暗示，改为明确复用 `owl/admin` 的上层能力。
- 文档不再使用“独立子系统最小模板/公开 CRUD”作为主叙事，避免和 `owl-admin` 实际架构冲突。
