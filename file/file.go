package file

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/J-guanghua/rwlock"
)

type rwFile struct {
	file *os.File
	name string
	m    sync.Mutex
}

func (file *rwFile) Lock(ctx context.Context) (err error) {
	err = file.acquireLock(ctx)
	if !errors.Is(err, rwlock.ErrFailed) {
		return err
	} else if err != nil {
	LoopLock:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1000 * time.Millisecond):
			err := file.acquireLock(ctx)
			if errors.Is(err, rwlock.ErrFailed) {
				goto LoopLock
			} else if err != nil {
				return err
			}
		}
	}
	return nil
}

func (file *rwFile) Unlock(ctx context.Context) error {
	return file.releaseLock(ctx)
}

// 获取文件句柄
func (file *rwFile) acquireLock(_ context.Context) error {
	file.m.Lock()
	return acquireLock(file.file)
}

// 释放文件锁
func (file *rwFile) releaseLock(_ context.Context) error {
	defer file.m.Unlock()
	return releaseLock(file.file)
}
