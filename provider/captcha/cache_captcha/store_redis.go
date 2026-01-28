package cache_captcha

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client redis.UniversalClient
}

// NewRedisStore 创建Redis存储实例
func NewRedisStore(client redis.UniversalClient) *RedisStore {
	return &RedisStore{client: client}
}

// Start Redis存储无需额外启动逻辑
func (s *RedisStore) Start() {}

// Save 保存验证码记录到Redis
func (s *RedisStore) Save(ctx context.Context, key string, record *CaptchaRecord, ttl time.Duration) error {
	b, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, b, ttl).Err()
}

// Load 从Redis加载验证码记录
func (s *RedisStore) Load(ctx context.Context, key string) (*CaptchaRecord, error) {
	val, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCaptchaNotFound
		}
		return nil, err
	}
	var record CaptchaRecord
	if err := json.Unmarshal([]byte(val), &record); err != nil {
		return nil, err
	}
	return &record, nil
}

// Remove 从Redis删除验证码记录
func (s *RedisStore) Remove(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}
