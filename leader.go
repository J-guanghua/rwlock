package rwlock

import (
	"context"
	"time"
)

type LeaderConfig struct {
	Name           string
	Renewaltime    time.Duration
	OnCallback     func(ctx context.Context)
	OnStopCallback func()
	ErrorCallback  func(err error)
}

func Leader(ctx context.Context, mutex Mutex, config LeaderConfig) {
	ctx, cancel := context.WithCancel(ctx)
	err := mutex.Lock(ctx)
	if err != nil {
		config.ErrorCallback(err)
	}
	defer cancel()
	defer mutex.Unlock(ctx)
	config.OnCallback(ctx)
}
