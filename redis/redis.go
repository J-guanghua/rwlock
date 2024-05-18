package redis

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/J-guanghua/rwlock"
	"github.com/go-redis/redis/v8"
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

type rwRedis struct {
	name   string
	sema   uint32
	wait   int32
	client *redis.Client
	signal chan struct{}
	opts   *rwlock.Options
	cancel context.CancelFunc
}

func (r *rwRedis) getOptions(ctx context.Context) *rwlock.Options {
	opts, ok := rwlock.FromContext(ctx)
	if ok {
		return opts
	}
	return r.opts
}

func (r *rwRedis) Lock(ctx context.Context) (err error) {
	if r.sema > 0 || r.wait > 0 {
		r.notify(rwlock.GetGoroutineID())
	} else if err = r.acquireLock(ctx); err == nil {
		return nil
	} else if !errors.Is(err, rwlock.ErrFailed) {
		return err
	}
	atomic.AddInt32(&r.wait, 1)
LoopLock:
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.signal:
		err = r.acquireLock(ctx)
		if errors.Is(err, rwlock.ErrFailed) {
			goto LoopLock
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (r *rwRedis) Unlock(ctx context.Context) error {
	r.cancel()
	atomic.AddInt32(&r.wait, -1)
	_, err := releaseScript.Eval(ctx, r.client, []string{r.name}, r.opts.Value).Result()
	atomic.StoreUint32(&r.sema, 0)
	r.notify(rwlock.GetGoroutineID())
	return err
}

func (r *rwRedis) acquireLock(ctx context.Context) error {
	opts := r.getOptions(ctx)
	expiry := int(opts.Expiry / time.Millisecond)
	result, err := acquireScript.Eval(ctx, r.client, []string{r.name}, opts.Value, expiry).Result()
	if err != nil {
		return err
	}
	if result == int64(1) {
		atomic.StoreUint32(&r.sema, uint32(rwlock.GetGoroutineID()))
		ctx, r.cancel = context.WithCancel(context.TODO())
		go r.touchRenewal(&rwlock.Renewal{Ctx: ctx, Name: r.name, Cancel: r.cancel})
		return nil
	} else if r.sema == 0 {
		r.notify(rwlock.GetGoroutineID())
	}
	return rwlock.ErrFailed
}

// 过期前重新开始（有效期的）延长
func (r *rwRedis) touchRenewal(touch *rwlock.Renewal) {
	opts := r.getOptions(touch.Ctx)
	for {
		select {
		case <-touch.Ctx.Done():
			return
		case <-time.After(opts.Expiry - 2000*time.Millisecond):
			expiry := int(opts.Expiry / time.Millisecond)
			result, err := touchScript.Eval(touch.Ctx,
				r.client, []string{touch.Name}, opts.Value, expiry).Result()
			touch.Err = err
			touch.Result = result == int64(1)
			r.opts.OnRenewal(touch)
		}
	}
}

func (r *rwRedis) notify(_ int64) {
	for i := 0; i <= len(r.signal); i++ {
		select {
		case r.signal <- struct{}{}:
		default:
			return
		}
	}
}
