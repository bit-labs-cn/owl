package cache_captcha

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

type memRecord struct {
	value     CaptchaRecord
	expiresAt time.Time
}

type MemoryStore struct {
	mu              sync.RWMutex
	items           map[string]memRecord
	cleanupInterval time.Duration
}

// NewMemoryStore 创建内存存储实例
func NewMemoryStore(interval time.Duration) *MemoryStore {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	return &MemoryStore{items: make(map[string]memRecord), cleanupInterval: interval}
}

// Start 启动过期数据清理协程
func (s *MemoryStore) Start() {
	go func() {
		ticker := time.NewTicker(s.cleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			s.mu.Lock()
			for key, item := range s.items {
				if now.After(item.expiresAt) {
					delete(s.items, key)
				}
			}
			s.mu.Unlock()
		}
	}()
}

// Save 保存验证码记录到内存
func (s *MemoryStore) Save(ctx context.Context, key string, record *CaptchaRecord, ttl time.Duration) error {
	_ = ctx
	if _, err := json.Marshal(record); err != nil {
		return err
	}
	exp := time.Now().Add(ttl)
	s.mu.Lock()
	s.items[key] = memRecord{value: *record, expiresAt: exp}
	s.mu.Unlock()
	return nil
}

// Load 从内存加载验证码记录
func (s *MemoryStore) Load(ctx context.Context, key string) (*CaptchaRecord, error) {
	_ = ctx
	s.mu.RLock()
	item, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrCaptchaNotFound
	}
	if time.Now().After(item.expiresAt) {
		_ = s.Remove(ctx, key)
		return nil, ErrCaptchaNotFound
	}
	return &item.value, nil
}

// Remove 从内存删除验证码记录
func (s *MemoryStore) Remove(ctx context.Context, key string) error {
	_ = ctx
	s.mu.Lock()
	delete(s.items, key)
	s.mu.Unlock()
	return nil
}
