package router

import (
	"bit-labs.cn/owl/utils"
	"github.com/jinzhu/copier"
)

type MenuType string

const (
	MenuTypeDir  MenuType = "目录"
	MenuTypeMenu MenuType = "菜单"
	MenuTypeBtn  MenuType = "按钮"
)

// Meta 菜单 meta 信息，用于前端显示， 需要参考 pure-admin 的文档，菜单是对 pure-admin 进行适配
type Meta struct {
	Title      string `json:"title"`              // 菜单标题
	Icon       string `json:"icon"`               // 菜单图标
	ShowLink   bool   `json:"showLink"`           // 是否显示菜单
	ShowParent bool   `json:"showParent"`         // 是否显示父级菜单
	NoLayout   bool   `json:"noLayout,omitempty"` // 是否跳过根布局（全屏独立页）
	Target     string `json:"target,omitempty"`   // 菜单打开目标（如 _blank 新标签页）
}

type Menu struct {
	ID                   string   `json:"id"`                   // 菜单唯一标识
	Path                 string   `json:"path"`                 // 前端路由地址
	Name                 string   `json:"name"`                 // 前端路由名称，组件名称
	ParentID             string   `json:"parentId"`             // 父级菜单ID
	Rank                 int      `json:"rank,omitempty"`       // 菜单排序
	Meta                 Meta     `json:"meta"`                 // 菜单 meta 信息，用于前端显示
	MenuType             MenuType `json:"menuType"`             // 菜单类型，菜单，按钮
	DependentsPermission []string `json:"dependentsPermission"` // 此动作需要拥有的api访问权限，如果是按钮，可以设置此字段
	Children             []*Menu  `json:"children,omitempty"`   // 子菜单
}

// Clone 复制菜单
func (i *Menu) Clone() *Menu {
	if i == nil {
		return nil
	}

	// 复制当前节点
	var cloneMenu Menu
	_ = copier.Copy(&cloneMenu, i)
	// 递归复制子节点
	if i.Children != nil {
		clonedChildren := make([]*Menu, len(i.Children))
		for i, child := range i.Children {
			clonedChildren[i] = child.Clone()
		}
		cloneMenu.Children = clonedChildren
	}

	return &cloneMenu
}

var staticMenus []*Menu

type MenuRepository struct {
}

func NewMenuRepository() *MenuRepository {
	return &MenuRepository{}
}

// AddMenu 添加菜单
func (m *MenuRepository) AddMenu(menus ...*Menu) {
	staticMenus = append(staticMenus, menus...)
}

// ChangeName 按路由 Name 递归查找菜单节点，将 Meta.Title 改为 newTitle；找到返回 true
func (m *MenuRepository) ChangeName(menuName, newTitle string) bool {
	return m.ChangeMeta(menuName, func(meta *Meta) {
		meta.Title = newTitle
	})
}

// ChangeMeta 按路由 Name 递归查找菜单节点，用 mutator 修改 Meta；找到返回 true
func (m *MenuRepository) ChangeMeta(menuName string, mutator func(meta *Meta)) bool {
	if mutator == nil {
		return false
	}
	for _, menu := range staticMenus {
		if changeMenuMeta(menu, menuName, mutator) {
			return true
		}
	}
	return false
}

func changeMenuMeta(menu *Menu, menuName string, mutator func(meta *Meta)) bool {
	if menu == nil {
		return false
	}
	if menu.Name == menuName {
		mutator(&menu.Meta)
		return true
	}
	for _, child := range menu.Children {
		if changeMenuMeta(child, menuName, mutator) {
			return true
		}
	}
	return false
}

// CloneMenus 复制菜单, 避免修改原菜单
func (m *MenuRepository) CloneMenus() []*Menu {
	var clonedMenus []*Menu

	for _, menu := range staticMenus {
		clonedMenus = append(clonedMenus, menu.Clone())
	}
	return clonedMenus
}

// GetMenuWithoutBtn 获取菜单，不包含按钮
func (m *MenuRepository) GetMenuWithoutBtn() []*Menu {
	clonedMenu := m.GetMenusWithBtn()
	for _, m2 := range clonedMenu {
		iteratorMenuWithoutBtn(m2, 1)
	}
	return clonedMenu
}

// GetMenuByMenuIDs 根据菜单id，返回菜单
func (m *MenuRepository) GetMenuByMenuIDs(menuIDs ...string) []*Menu {
	if len(menuIDs) == 0 {
		return nil
	}
	clonedMenu := m.GetMenusWithBtn()
	for _, m2 := range clonedMenu {
		iteratorMenuSetShowLink(m2, menuIDs...)
	}

	for _, menu := range clonedMenu {
		iteratorMenuWithoutBtn(menu, 1)
	}

	return clonedMenu
}

// GetMenusWithBtn 获取菜单，包含按钮
func (m *MenuRepository) GetMenusWithBtn() []*Menu {
	clonedMenu := m.CloneMenus()
	for _, m2 := range clonedMenu {
		iteratorMenu(m2, 1)
	}
	return clonedMenu
}

// GetPermissionsByMenuIDs 根据菜单的id，返回所有的权限
func (m *MenuRepository) GetPermissionsByMenuIDs(ids ...string) []string {
	var permissions []string
	menus := m.GetMenusWithBtn()
	for _, menu := range menus {
		permissions = append(permissions, iteratorGetPermission(menu, ids...)...)
	}
	return utils.Unique(permissions)
}

func iteratorMenuWithoutBtn(menu *Menu, level int) {
	if level == 1 {
		menu.ID = menu.Name
	}

	// 创建一个新的切片来保存过滤后的子节点
	filteredChildren := make([]*Menu, 0)
	for _, v := range menu.Children {
		if v.MenuType == MenuTypeBtn {
			continue
		}
		v.ID = menu.ID + "," + v.Name
		v.ParentID = menu.Name

		iteratorMenuWithoutBtn(v, level+1)

		filteredChildren = append(filteredChildren, v)

	}
	menu.Children = filteredChildren
}

func iteratorMenu(menu *Menu, level int) {
	//menu.Meta.ShowLink = true
	menu.Meta.ShowParent = true
	if level == 1 {
		menu.ID = menu.Name
	}
	for _, v := range menu.Children {
		v.ID = menu.ID + "," + v.Name
		v.ParentID = menu.Name
		iteratorMenu(v, level+1)
	}
}

// iteratorGetPermission 迭代菜单获取权限
func iteratorGetPermission(menu *Menu, ids ...string) []string {
	var permissions []string
	for _, id := range ids {
		if menu.ID == id {
			permissions = append(permissions, menu.DependentsPermission...)
		}
	}

	if menu.Children != nil && len(menu.Children) > 0 {
		for _, v := range menu.Children {
			subPermissions := iteratorGetPermission(v, ids...)
			permissions = append(permissions, subPermissions...)
		}
	}
	return permissions
}

func iteratorMenuSetShowLink(menu *Menu, ids ...string) bool {

	var show bool
	for _, id := range ids {
		if menu.ID == id {
			menu.Meta.ShowLink = true
			show = true
			break
		}
	}
	for _, v := range menu.Children {

		x := iteratorMenuSetShowLink(v, ids...) // ↑
		if x {
			show = true
		}
	}
	// 当前节点自身命中或任一子节点命中，都应显示
	menu.Meta.ShowLink = show
	return show
}
