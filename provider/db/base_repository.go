package db

import (
	"context"

	"gorm.io/gorm"
)

type ChangeStatus struct {
	ID     uint `json:"id,string" binding:"required"`
	Status int  `json:"status"`
}

type BaseRepository[T any] struct {
	db  *gorm.DB
	ctx context.Context
}

func NewBaseRepository[T any](db *gorm.DB) BaseRepository[T] {
	return BaseRepository[T]{
		db: db,
	}
}

// Save 保存或者更新， save 会保存所有字段，包括零值字段
func (i *BaseRepository[T]) Save(data *T) error {
	return i.db.Save(data).Error
}

// Delete 删除数据
func (i *BaseRepository[T]) Delete(ids ...any) error {
	return i.db.Model(new(T)).Where("id in ?", ids).Delete(nil).Error
}

func (i *BaseRepository[T]) WithContext(ctx context.Context) *BaseRepository[T] {
	i.ctx = ctx
	return i
}

// Detail 获取详情
func (i *BaseRepository[T]) Detail(id any) (*T, error) {
	model := new(T)
	db := i.db.Model(model).Where("id", id).First(model)
	return model, db.Error
}

// Retrieve 获取详情
func (i *BaseRepository[T]) Retrieve(page, pageSize int, fn func(db *gorm.DB)) (count int64, list []T, err error) {
	var model T
	newDB := i.db.WithContext(i.ctx).Model(model)
	if fn != nil {
		fn(newDB)
	}
	newDB.Count(&count)
	err = newDB.Scopes(Paginate(page, pageSize)).Find(&list).Error
	return
}

// Unique 唯一性判断
func (i *BaseRepository[T]) Unique(id uint, fn func(db *gorm.DB)) (*T, bool) {
	model := new(T)
	db := i.db.Model(model)
	var count int64
	fn(db)
	if id > 0 {
		db.Where("id != ?", id).Count(&count).Find(&model)
	} else {
		db.Count(&count).Find(&model)
	}
	if count > 0 {
		return model, true
	}
	return model, false
}

// ChangeStatus 更改状态，我们经常会需要单独的更改状态，比如禁用，启用等。
func (i *BaseRepository[T]) ChangeStatus(req *ChangeStatus) error {
	return i.db.Model(new(T)).
		Where("id", req.ID).
		Update("status", req.Status).Error
}
