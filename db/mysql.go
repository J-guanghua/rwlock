package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/J-guanghua/rwlock"
)

type rwMysql struct {
	db     *sql.DB
	name   string
	sema   uint32
	wait   int32
	opts   *rwlock.Options
	signal chan struct{}
}

func (rw *rwMysql) getOptions(ctx context.Context) *rwlock.Options {
	if opts, ok := rwlock.FromContext(ctx); ok {
		return opts
	}
	return rw.opts
}

func (rw *rwMysql) Lock(ctx context.Context) error {
	var err error
	options := rw.getOptions(ctx)
	if rw.sema == 1 || rw.wait > 0 {
		rw.notify(rwlock.GetGoroutineID())
	} else if err = rw.acquireLock(ctx); err == nil {
		return nil
	} else if !errors.Is(err, rwlock.ErrFailed) {
		return err
	}
	var tries int
	atomic.AddInt32(&rw.wait, 1)
LoopLock:
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-rw.signal:
		tries++
		err = rw.acquireLock(ctx)
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

func (rw *rwMysql) Unlock(ctx context.Context) error {
	defer rw.notify(rwlock.GetGoroutineID())
	defer atomic.StoreUint32(&rw.sema, 0)
	_ = atomic.AddInt32(&rw.wait, -1)
	return rw.releaseUnlock(ctx)
}

func (rw *rwMysql) acquireLock(ctx context.Context) error {
	row, err := rw.db.QueryContext(ctx, "SELECT GET_LOCK(?,?)", rw.name, 4)
	if err != nil {
		return err
	}
	var result int
	defer row.Close()
	if row.Next() {
		err = row.Scan(&result)
		if err != nil {
			return err
		} else if result == 1 {
			atomic.StoreUint32(&rw.sema, 1)
			return nil
		} else if rw.sema == 0 {
			rw.notify(rwlock.GetGoroutineID())
		}
		return rwlock.ErrFailed
	}
	return row.Err()
}

func (rw *rwMysql) releaseUnlock(ctx context.Context) error {
	// 释放锁
	row, err := rw.db.QueryContext(ctx, "SELECT RELEASE_LOCK(?)", rw.name)
	if err != nil {
		return err
	}
	var result int
	defer row.Close()
	defer rw.notify(rwlock.GetGoroutineID())
	if row.Next() {
		err = row.Scan(&result)
		if err != nil {
			return err
		} else if result == 1 {
			atomic.StoreUint32(&rw.sema, 0)
			return nil
		}
	}
	return row.Err()
}

func (rw *rwMysql) notify(_ int64) {
	for i := 0; i <= len(rw.signal); i++ {
		select {
		case rw.signal <- struct{}{}:
		default:
			return
		}
	}
}
