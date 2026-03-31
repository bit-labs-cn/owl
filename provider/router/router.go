package router

import (
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

type AccessLevel string

const (
	AccessPublic        AccessLevel = "开放接口"
	AccessAuthenticated AccessLevel = "需要登录"
	AccessAuthorized    AccessLevel = "需要授权"
	AccessSuperAdmin    AccessLevel = "仅超管"
)

type RouterInfo struct {
	Group            string          `json:"group"`            // 接口分组
	Method           string          `json:"method"`           // 请求方式
	Path             string          `json:"path"`             // 路由路径
	PathWithoutGroup string          `json:"pathWithoutGroup"` // 路由路径（不包含分组）
	Name             string          `json:"name"`             // 路由名称
	Module           string          `json:"module"`           // 模块名称
	Permission       string          `json:"permission"`       // 权限标识
	Description      string          `json:"description"`      // 描述
	AccessLevel      AccessLevel     `json:"accessLevel"`      // 访问级别
	handle           gin.HandlerFunc // 处理函数
	operateLog       bool            // 是否记录操作日志
}

// 全局路由注册表，用于保存所有注册进来的路由
var (
	routesLock       sync.RWMutex
	registeredRoutes []RouterInfo
)

// RegisterRoute 保存已注册的路由信息
func RegisterRoute(info *RouterInfo) {
	if info == nil {
		return
	}
	routesLock.Lock()
	// 保存副本，避免外部修改影响内部存储
	registeredRoutes = append(registeredRoutes, *info)
	routesLock.Unlock()
}

// GetAllRoutes 返回所有已注册的路由信息（副本）
func GetAllRoutes() []RouterInfo {
	routesLock.RLock()
	defer routesLock.RUnlock()
	result := make([]RouterInfo, len(registeredRoutes))
	copy(result, registeredRoutes)
	return result
}

type Dep struct {
	Handler Handler
	Method  gin.HandlerFunc
}
type RouterInfoBuilder struct {
	menu     *Menu
	moduleEn string // 模块英文名称
	moduleZh string // 模块中文名称
	appName  string
	router   *gin.RouterGroup
	handler  Handler // 定义 handler，可以获取 handler 的 moduleName
	lock     sync.RWMutex

	currentRouter            *RouterInfo
	currentRouterMiddlewares []gin.HandlerFunc
	deps                     []Dep
}

type MenuOption struct {
	ComponentName string // 组件名称
	Path          string // 路由地址
	Icon          string // 图标
}

func NewRouteInfoBuilder(appName string, handle Handler, router *gin.RouterGroup, meta MenuOption) *RouterInfoBuilder {
	en, zh := handle.ModuleName()
	return &RouterInfoBuilder{
		moduleEn: en,
		moduleZh: zh,
		router:   router,
		handler:  handle,
		appName:  appName,
		menu: &Menu{
			Name: meta.ComponentName,
			Path: meta.Path,
			Meta: Meta{
				Title:      zh,
				Icon:       meta.Icon,
				ShowLink:   true,
				ShowParent: true,
			},
			MenuType: MenuTypeMenu,
		},
	}
}

func (i *RouterInfoBuilder) add(method, path string, accessLevel AccessLevel, handle gin.HandlerFunc) *RouterInfoBuilder {
	i.lock.Lock()
	i.currentRouter = &RouterInfo{
		Group:            i.router.BasePath(),
		Method:           method,
		Path:             i.router.BasePath() + path,
		PathWithoutGroup: path,
		Module:           i.moduleZh,
		AccessLevel:      accessLevel,
		handle:           handle,
		operateLog:       method != http.MethodGet,
	}

	return i
}

func (i *RouterInfoBuilder) getPermissionStr(funcName string) string {
	return i.appName + ":" + i.moduleEn + ":" + funcName
}

// Build 构建路由,以及菜单
func (i *RouterInfoBuilder) Build() {
	defer i.lock.Unlock()

	// 设置路由授权标识，前端可用于控制按钮的显示
	handleName := nameOfFunction(i.currentRouter.handle)
	i.currentRouter.Permission = i.getPermissionStr(handleName)

	var permissions []string
	permissions = append(permissions, i.currentRouter.Permission)

	// 设置接口依赖，完成一个动作可能会需要很多个接口配合
	for _, dep := range i.deps {
		funcName := nameOfFunction(dep.Method)
		en, _ := dep.Handler.ModuleName()
		permission := i.appName + ":" + en + ":" + funcName
		permissions = append(permissions, permission)
	}
	i.deps = nil // 清理依赖

	// 构建菜单（按钮）
	if i.currentRouter.AccessLevel == AccessAuthorized {
		i.menu.Children = append(i.menu.Children, &Menu{
			Name: handleName,
			Meta: Meta{
				Title: i.currentRouter.Name,
			},
			MenuType:             MenuTypeBtn,
			DependentsPermission: permissions,
		})
	}
	// 添加路由
	handlers := append(i.currentRouterMiddlewares, i.currentRouter.handle)
	i.router.Handle(i.currentRouter.Method, i.currentRouter.PathWithoutGroup, handlers...)
	i.currentRouterMiddlewares = nil

	// 保存路由到全局注册表
	RegisterRoute(i.currentRouter)
}

func (i *RouterInfoBuilder) Description(d string) *RouterInfoBuilder {
	i.currentRouter.Description = d
	return i
}

func (i *RouterInfoBuilder) Name(n string) *RouterInfoBuilder {
	i.currentRouter.Name = n
	return i
}

func (i *RouterInfoBuilder) WithoutOperateLog() *RouterInfoBuilder {
	i.currentRouter.operateLog = false
	return i
}

func (i *RouterInfoBuilder) WithOperateLog() *RouterInfoBuilder {
	i.currentRouter.operateLog = true
	return i
}

// Deps 此接口依赖的其他接口，比如说修改用户，则需要 `获取用户详情` `更新用户`  两个接口
func (i *RouterInfoBuilder) Deps(dep ...Dep) *RouterInfoBuilder {
	i.deps = append(i.deps, dep...)
	return i
}
func (i *RouterInfoBuilder) Post(path string, accessLevel AccessLevel, handle gin.HandlerFunc) *RouterInfoBuilder {
	return i.add(http.MethodPost, path, accessLevel, handle)
}
func (i *RouterInfoBuilder) Put(path string, accessLevel AccessLevel, handle gin.HandlerFunc) *RouterInfoBuilder {
	return i.add(http.MethodPut, path, accessLevel, handle)
}
func (i *RouterInfoBuilder) Delete(path string, accessLevel AccessLevel, handle gin.HandlerFunc) *RouterInfoBuilder {
	return i.add(http.MethodDelete, path, accessLevel, handle)
}
func (i *RouterInfoBuilder) Get(path string, accessLevel AccessLevel, handle gin.HandlerFunc) *RouterInfoBuilder {
	return i.add(http.MethodGet, path, accessLevel, handle)
}

func (i *RouterInfoBuilder) Use(handle ...gin.HandlerFunc) *RouterInfoBuilder {
	i.currentRouterMiddlewares = append(i.currentRouterMiddlewares, handle...)
	return i
}

func (i *RouterInfoBuilder) GetMenu() *Menu {
	return i.menu
}

func (i RouterInfo) ShouldOperateLog() bool {
	return i.operateLog
}

// 获取函数名
func nameOfFunction(f any) string {
	fullName := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	// bit-labs.cn/owl-admin/app/handle/v1.(*RoleHandle).FindById-fm
	lastIndex := strings.LastIndex(fullName, ".")
	if lastIndex > 0 {
		return strings.Replace(fullName[lastIndex+1:], "-fm", "", 1)
	}
	return fullName
}
