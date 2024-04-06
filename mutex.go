package mutex

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"
)

var ErrFail = errors.New("fiand error")

var global Lock

func SetDefaultMutex(golbalLock Lock) {
	global = golbalLock
}

type Lock interface {
	NewMutex(ctx context.Context, name string, option ...Option) Mutex
}

func NewMutex(ctx context.Context, name string) Mutex {
	return global.NewMutex(ctx, name)
}

type Mutex interface {
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error
}

// getGoroutineID 获取当前 goroutine 的 ID
func GetGoroutineID() int64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	var id int64
	fmt.Sscanf(string(b), "goroutine %d ", &id)
	return id
}

type Option func(options *Options)
type Options struct {
	Value  string
	Expiry time.Duration
	Touchf func(ctx context.Context, name string) time.Duration
}

func WithValue(v string) Option {
	return func(ops *Options) {
		ops.Value = v
	}
}
func WithExpiry(expiry time.Duration) Option {
	return func(ops *Options) {
		ops.Expiry = expiry
	}
}
func WithTouchf(f func(ctx context.Context, name string) time.Duration) Option {
	return func(ops *Options) {
		ops.Touchf = f
	}
}
