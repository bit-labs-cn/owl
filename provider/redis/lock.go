package redis

import (
	"context"
	"strings"
	"time"

	redsync "github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/redis/go-redis/v9"
)

type Locker interface {
	Lock(key string) error
	Unlock()
}

type LockerFactory interface {
	New() Locker
}

type RedisLockerFactory struct {
	rs *redsync.Redsync
}

func NewRedisLockerFactory(client redis.UniversalClient) *RedisLockerFactory {
	switch c := client.(type) {
	case *redis.Client:
		return &RedisLockerFactory{rs: redsync.New(goredis.NewPool(c))}
	case *redis.ClusterClient:
		return &RedisLockerFactory{rs: redsync.New(goredis.NewPool(c))}
	default:
		return &RedisLockerFactory{rs: redsync.New(goredis.NewPool(c.(*redis.Client)))}
	}
}

type redisLocker struct {
	rs    *redsync.Redsync
	mutex *redsync.Mutex
}

func (f *RedisLockerFactory) New() Locker {
	return &redisLocker{rs: f.rs}
}

func (l *redisLocker) Lock(key string) error {
	expiry := 10 * time.Second
	if strings.HasSuffix(key, ":import") {
		expiry = 3 * time.Minute
	}
	l.mutex = l.rs.NewMutex(key,
		redsync.WithExpiry(expiry),
		redsync.WithTries(3),
		redsync.WithRetryDelay(200*time.Millisecond),
	)
	return l.mutex.LockContext(context.Background())
}

func (l *redisLocker) Unlock() {
	if l.mutex != nil {
		_, _ = l.mutex.Unlock()
		l.mutex = nil
	}
}
