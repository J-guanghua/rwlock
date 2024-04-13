package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/J-guanghua/rwlock"
	"log"
	"sync"
	"sync/atomic"
)

type rwMysql struct {
	db     *sql.DB
	name   string
	sema   uint32
	wait   int32
	mtx    sync.Mutex
	ops    *rwlock.Options
	cancel context.CancelFunc
	signal chan struct{}
}

func (db *rwMysql) Lock(ctx context.Context) error {
	var err error
	if db.sema == 1 || db.wait > 0 {
		db.notify(rwlock.GetGoroutineID())
	} else if err = db.acquireLock(ctx); err == nil {
		return nil
	} else if !errors.Is(err, rwlock.ErrFail) {
		return err
	}
	atomic.AddInt32(&db.wait, 1)
LoopLock:
	select {
	case <-db.signal:
		err = db.acquireLock(ctx)
		if errors.Is(err, rwlock.ErrFail) {
			goto LoopLock
		} else if err != nil {
			log.Println("acquireLock",err)
			return err
		}
	}
	return nil
}

func (db *rwMysql) Unlock(ctx context.Context) error {
	defer db.notify(rwlock.GetGoroutineID())
	defer atomic.StoreUint32(&db.sema, 0)
	atomic.AddInt32(&db.wait, -1)
	log.Println("Unlock...",db.sema,db.wait)
	if err := db.releaseUnlock(ctx); err != nil {
		return err
	}
	return nil
}

func (db *rwMysql) acquireLock(ctx context.Context) error {
	row, err := db.db.QueryContext(ctx, "SELECT GET_LOCK(?,?)", db.name, 4)
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
			atomic.StoreUint32(&db.sema, 1)
			return nil
		} else if db.sema == 0 {
			db.notify(rwlock.GetGoroutineID())
		}
		return rwlock.ErrFail
	}
	return row.Err()
}

func (db *rwMysql) releaseUnlock(ctx context.Context) error {
	// 释放锁
	row, err := db.db.QueryContext(ctx, "SELECT RELEASE_LOCK(?)", db.name)
	if err != nil {
		return err
	}
	var result int
	defer row.Close()
	defer db.notify(rwlock.GetGoroutineID())
	if row.Next() {
		err = row.Scan(&result)
		if err != nil {
			return err
		} else if result == 1 {
			atomic.StoreUint32(&db.sema, 0)
			return nil
		}
	}
	return row.Err()
}

func (db *rwMysql) notify(gid int64) {
	for i := 0; i <= len(db.signal); i++ {
		select {
		case db.signal <- struct{}{}:
		default:
			return
		}
	}
}
