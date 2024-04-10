package rwlock

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"
)

var ErrFail = errors.New("fiand error")

type Mutex interface {
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error
}

type RWMutex interface {
	Mutex
	RLock(ctx context.Context) error
	RUnlock(ctx context.Context) error
}

func GetGoroutineID() int64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	var id int64
	fmt.Sscanf(string(b), "goroutine %d ", &id)
	return id
}

type Touch struct {
	Ctx    context.Context
	Cancel context.CancelFunc
	Err    error
}

type Options struct {
	Value  string
	Expiry time.Duration
	Touchf func(context.Context, context.CancelFunc) time.Duration
}

type Option func(options *Options)

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
func WithTouchf(f func(context.Context, context.CancelFunc) time.Duration) Option {
	return func(ops *Options) {
		ops.Touchf = f
	}
}
