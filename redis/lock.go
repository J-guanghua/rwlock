package redis

import (
	"context"
	"github.com/J-guanghua/mutex"
	"github.com/go-redis/redis/v8"
	"sync"
	"time"
)

type redisLock struct {
	mtx   sync.Mutex
	pool  []*redis.Client
	mutex map[string]*rMutex
}

func NewRedisLock(pools ...*redis.Client) *redisLock {
	return &redisLock{
		pool: pools,
	}
}

func (rlock *redisLock) allocation(name string, opts *mutex.Options) mutex.Mutex {
	rlock.mtx.Lock()
	defer rlock.mtx.Unlock()
	if rlock.mutex == nil {
		rlock.mutex = make(map[string]*rMutex, 100)
	}
	if rlock.mutex[name] == nil {
		index := len(rlock.mutex) % len(rlock.pool)
		rlock.mutex[name] = &rMutex{
			name:   name,
			opts:   opts,
			client: rlock.pool[index],
			signal: make(chan struct{}, 1),
		}
	}
	return rlock.mutex[name]
}

func (rlock *redisLock) NewMutex(ctx context.Context, name string, opts ...mutex.Option) mutex.Mutex {
	opt := &mutex.Options{
		Expiry: 5 * time.Second,
		Value:  "default",
	}
	for _, o := range opts {
		o(opt)
	}
	return rlock.allocation(name, opt)
}
