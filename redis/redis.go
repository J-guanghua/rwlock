package redis

import (
	"context"
	"errors"
	"github.com/J-guanghua/mutex"
	"github.com/go-redis/redis/v8"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

var (
	touchScript = redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)
	// Lua 脚本，用于尝试获取锁并设置过期时间
	acquireScript = redis.NewScript(`
		if redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2], "NX") then
			return 1
		else
			return 0
		end
	`)
	// Lua 脚本，用于释放锁
	releaseScript = redis.NewScript(`
		local val = redis.call("GET", KEYS[1])
		if val == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		elseif val == false then
			return -1
		else
			return 0
		end
	`)
)

type rMutex struct {
	name   string
	mtx    sync.Mutex
	sema   uint32
	client *redis.Client
	signal chan struct{}
	//starving chan struct{}
	opts   *mutex.Options
	cancel context.CancelFunc
}

func (r *rMutex) Lock(ctx context.Context) (err error) {
	if err = r.acquirLock(ctx); err == nil {
		return nil
	} else if !errors.Is(err, mutex.ErrFail) {
		return err
	}
LoopLock:
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.signal:
		err = r.acquirLock(ctx)
		if errors.Is(err, mutex.ErrFail) {
			goto LoopLock
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (r *rMutex) Unlock(ctx context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	_, err := releaseScript.Run(ctx, r.client, []string{r.name}, r.opts.Value).Result()
	//log.Printf("Unlock: %v, err :%v(%v) ", r.name, err, i)
	atomic.StoreUint32(&r.sema, 0)
	r.notify(mutex.GetGoroutineID())
	return err
}

func (r *rMutex) acquirLock(ctx context.Context) error {
	name := []string{r.name}
	expiry := int(r.opts.Expiry / time.Millisecond)
	result, err := acquireScript.Run(ctx, r.client, name, r.opts.Value, expiry).Result()
	if err != nil {
		return err
	}
	if result == int64(1) {
		atomic.StoreUint32(&r.sema, 1)
		if r.opts.Touchf != nil {
			ctx, r.cancel = context.WithCancel(ctx)
			go r.touchRenewal(ctx, r.name)
		}
		return nil
	} else if r.sema == 0 {
		r.notify(mutex.GetGoroutineID())
	}
	return mutex.ErrFail
}

// 过期前询问是否续签时间
func (r *rMutex) touchRenewal(ctx context.Context, name string) (bool, error) {
	select {
	case <-ctx.Done():
		log.Printf("解除续期:%v;", r.name)
		return false, nil
	case <-time.After(r.opts.Expiry - 500*time.Millisecond):
		if duration := r.opts.Touchf(ctx, name); duration > 0 {
			expiry := int(duration / time.Millisecond)
			result, err := touchScript.Run(ctx,
				r.client, []string{name}, r.opts.Value, expiry).Result()
			//log.Printf("续签:%v , result:%v;err %v", expiry, result, err)
			if err != nil {
				return false, err
			}
			return result != int64(0), nil
		}
	}
	return false, nil
}

func (r *rMutex) notify(gid int64) {
	//log.Printf("notify:  Gid:%v,signal-%v", gid, len(r.signal))
	for i := 0; i <= len(r.signal); i++ {
		select {
		case r.signal <- struct{}{}:
		default:
			return
		}
	}
}
