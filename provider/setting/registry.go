package setting

import (
	"fmt"
	"strings"
)

// Registry 保存本次进程里各子系统登记的配置。启动完成后只读。
type Registry struct {
	list    []Subsystem
	byName  map[string]int
	itemAt  map[string]map[string]Item
	groupAt map[string]map[string]Group
}

// NewRegistry 返回一份空注册表。
func NewRegistry() *Registry {
	return &Registry{
		byName:  map[string]int{},
		itemAt:  map[string]map[string]Item{},
		groupAt: map[string]map[string]Group{},
	}
}

// Add 登记一个子系统。键重复、默认值不合法时返回错误。
func (r *Registry) Add(name string, catalog Catalog) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("setting: subsystem name is empty")
	}
	if _, ok := r.byName[name]; ok {
		return fmt.Errorf("setting: subsystem %q already registered", name)
	}
	title := strings.TrimSpace(catalog.Title)
	if title == "" {
		return fmt.Errorf("setting: subsystem %q title is empty", name)
	}

	groups := make([]Group, 0, len(catalog.Groups))
	items := map[string]Item{}
	owners := map[string]Group{}
	seenGroup := map[string]struct{}{}

	for _, group := range catalog.Groups {
		prepared, err := absorbGroup(name, group, 1, items, seenGroup, owners)
		if err != nil {
			return err
		}
		groups = append(groups, prepared)
	}

	r.byName[name] = len(r.list)
	r.itemAt[name] = items
	r.groupAt[name] = owners
	r.list = append(r.list, Subsystem{Name: name, Title: title, Groups: groups})
	return nil
}

func absorbGroup(name string, group Group, depth int, items map[string]Item, seenGroup map[string]struct{}, owners map[string]Group) (Group, error) {
	groupKey := strings.TrimSpace(group.Key)
	groupTitle := strings.TrimSpace(group.Title)
	if groupKey == "" || groupTitle == "" {
		return Group{}, fmt.Errorf("setting: subsystem %q has a group without key or title", name)
	}
	if _, ok := seenGroup[groupKey]; ok {
		return Group{}, fmt.Errorf("setting: subsystem %q duplicate group %q", name, groupKey)
	}
	seenGroup[groupKey] = struct{}{}

	if len(group.Groups) > 0 {
		if depth >= 2 {
			return Group{}, fmt.Errorf("setting: subsystem %q group %q cannot contain another group", name, groupKey)
		}
		if len(group.Items) > 0 {
			return Group{}, fmt.Errorf("setting: subsystem %q group %q cannot hold both items and groups", name, groupKey)
		}
		children := make([]Group, 0, len(group.Groups))
		for _, child := range group.Groups {
			prepared, err := absorbGroup(name, child, depth+1, items, seenGroup, owners)
			if err != nil {
				return Group{}, err
			}
			children = append(children, prepared)
		}
		return Group{Key: groupKey, Title: groupTitle, Groups: children}, nil
	}
	if len(group.Items) == 0 {
		return Group{}, fmt.Errorf("setting: subsystem %q group %q has no items", name, groupKey)
	}

	cloned := make([]Item, len(group.Items))
	values := make(map[string]string, len(cloned))
	for i := range group.Items {
		item := group.Items[i]
		item.Key = strings.TrimSpace(item.Key)
		item.Title = strings.TrimSpace(item.Title)
		if item.Key == "" || item.Title == "" {
			return Group{}, fmt.Errorf("setting: subsystem %q group %q has an item without key or title", name, groupKey)
		}
		if _, ok := items[item.Key]; ok {
			return Group{}, fmt.Errorf("setting: subsystem %q duplicate key %q", name, item.Key)
		}
		if item.Min < 0 || item.Max < 0 {
			return Group{}, fmt.Errorf("setting: subsystem %q key %q has a negative bound", name, item.Key)
		}
		if item.Min != 0 && item.Max != 0 && item.Min > item.Max {
			return Group{}, fmt.Errorf("setting: subsystem %q key %q has min greater than max", name, item.Key)
		}
		if item.Type == TypeImage && (item.Min != 0 || item.Max > ImageMaxBytes) {
			return Group{}, fmt.Errorf("setting: subsystem %q key %q image limit must be within 200KB", name, item.Key)
		}
		if err := item.Accepts(item.Default); err != nil {
			return Group{}, fmt.Errorf("setting: subsystem %q key %q default: %w", name, item.Key, err)
		}
		cloned[i] = item
		items[item.Key] = item
		values[item.Key] = item.Default
	}
	leaf := Group{Key: groupKey, Title: groupTitle, Items: cloned, Validate: group.Validate}
	if leaf.Validate != nil {
		if err := leaf.Validate(values); err != nil {
			return Group{}, fmt.Errorf("setting: subsystem %q group %q: %w", name, groupKey, err)
		}
	}
	for _, item := range cloned {
		owners[item.Key] = leaf
	}
	return leaf, nil
}

// Subsystems 按登记顺序返回子系统。
func (r *Registry) Subsystems() []Subsystem {
	return r.list
}

// Find 按子系统标识查找。
func (r *Registry) Find(name string) (Subsystem, bool) {
	idx, ok := r.byName[name]
	if !ok {
		return Subsystem{}, false
	}
	return r.list[idx], true
}

// Item 按子系统和配置键查找。
func (r *Registry) Item(subsystem, key string) (Item, bool) {
	item, ok := r.itemAt[subsystem][key]
	return item, ok
}

// GroupOf 返回配置键所在的分组。嵌套时返回直接容纳该项的二级分组。
func (r *Registry) GroupOf(subsystem, key string) (Group, bool) {
	group, ok := r.groupAt[subsystem][key]
	return group, ok
}
