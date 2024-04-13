package redis

import (
	"context"
	"errors"
	"github.com/J-guanghua/rwlock"
	"github.com/go-redis/redis/v8"
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
	touchWithSetNXScript = redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		elseif redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2], "NX") then
			return 1
		else
			return 0
		end
`)
)

type rwRedis struct {
	name   string
	sema   uint32
	wait   int32
	mtx    sync.Mutex
	client *redis.Client
	signal chan struct{}
	//starving chan struct{}
	opts   *rwlock.Options
	cancel context.CancelFunc
}

func (r *rwRedis) Lock(ctx context.Context) (err error) {
	if r.sema == 1 || r.wait > 0 {
		r.notify(rwlock.GetGoroutineID())
	} else if err = r.acquirLock(ctx); err == nil {
		return nil
	} else if !errors.Is(err, rwlock.ErrFail) {
		return err
	}
	atomic.AddInt32(&r.wait, 1)
LoopLock:
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.signal:
		err = r.acquirLock(ctx)
		if errors.Is(err, rwlock.ErrFail) {
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

func (r *rwRedis) acquirLock(ctx context.Context) error {
	expiry := int(r.opts.Expiry / time.Millisecond)
	result, err := acquireScript.Eval(ctx, r.client, []string{r.name}, r.opts.Value, expiry).Result()
	if err != nil {
		return err
	}
	if result == int64(1) {
		atomic.StoreUint32(&r.sema, 1)
		ctx, r.cancel = context.WithCancel(context.TODO())
		go r.touchRenewal(&rwlock.Renewal{Ctx: ctx, Name: r.name, Cancel: r.cancel, Value: r.opts.Value})
		return nil
	} else if r.sema == 0 {
		r.notify(rwlock.GetGoroutineID())
	}
	//log.Printf("重试: %v , %v", r.name, err)
	return rwlock.ErrFail
}

// 过期前重新开始（有效期的）延长
func (r *rwRedis) touchRenewal(touch *rwlock.Renewal) {
	for {
		select {
		case <-touch.Ctx.Done():
			return
		case <-time.After(r.opts.Expiry - 2000*time.Millisecond):
			expiry := int(r.opts.Expiry / time.Millisecond)
			result, err := touchWithSetNXScript.Eval(touch.Ctx,
				r.client, []string{touch.Name}, r.opts.Value, expiry).Result()
			touch.Err = err
			touch.Result = result == int64(1)
			r.opts.OnRenewal(touch)
		}
	}
}

func (r *rwRedis) notify(gid int64) {
	for i := 0; i <= len(r.signal); i++ {
		select {
		case r.signal <- struct{}{}:
		default:
			return
		}
	}
}
