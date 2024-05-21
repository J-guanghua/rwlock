package redis

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/J-guanghua/rwlock"
	"github.com/go-redis/redis/v8"
)

var (
	// Lua 脚本，用于设置锁续签时长
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
	if opts, ok := rwlock.FromContext(ctx); ok {
		return opts
	}
	return r.opts
}

func (r *rwRedis) Lock(ctx context.Context) error {
	var err error
	options := r.getOptions(ctx)
	if r.sema > 0 || r.wait > 0 {
		r.notify(rwlock.GetGoroutineID())
	} else if err = r.acquireLock(ctx, options); err == nil {
		return nil
	} else if !errors.Is(err, rwlock.ErrFailed) {
		return err
	}
	var tries int
	atomic.AddInt32(&r.wait, 1)
LoopLock:
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.signal:
		tries++
		err = r.acquireLock(ctx, options)
		if errors.Is(err, rwlock.ErrFailed) {
			if options.Tries > 0 && tries >= options.Tries {
				return fmt.Errorf("尝试 %d 次,获取锁失败", tries)
			}
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

// 尝试获取锁，如果获取失败 通知到休眠协程
func (r *rwRedis) acquireLock(ctx context.Context, opts *rwlock.Options) error {
	expiry := int(opts.Expiry / time.Millisecond)
	result, err := acquireScript.Eval(ctx, r.client, []string{r.name}, opts.Value, expiry).Result()
	if err != nil {
		return err
	}
	if result == int64(1) {
		ctx, r.cancel = context.WithCancel(ctx)
		atomic.StoreUint32(&r.sema, uint32(rwlock.GetGoroutineID()))
		go r.touchRenewal(&rwlock.Renewal{Ctx: ctx, Name: r.name, Cancel: r.cancel})
		return nil
	} else if r.sema == 0 {
		r.notify(rwlock.GetGoroutineID())
	}
	return rwlock.ErrFailed
}

// 过期前 设置锁续签时长
func (r *rwRedis) touchRenewal(renewal *rwlock.Renewal) {
	opts := r.getOptions(renewal.Ctx)
	for {
		select {
		case <-renewal.Ctx.Done():
			return
		case <-time.After(opts.Expiry - 2000*time.Millisecond):
			expiry := int(opts.Expiry / time.Millisecond)
			result, err := touchScript.Eval(renewal.Ctx,
				r.client, []string{renewal.Name}, opts.Value, expiry).Result()
			renewal.Err = err
			renewal.Result = result == int64(1)
			r.opts.OnRenewal(renewal)
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
