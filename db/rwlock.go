package db

import (
	"database/sql"
	"sync"

	"github.com/J-guanghua/rwlock"
)

var dlock rwLock

func Init(dbs ...*sql.DB) {
	for _, db := range dbs {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		db.SetConnMaxLifetime(0)
		db.SetConnMaxIdleTime(0)
	}
	dlock.size = len(dbs)
	dlock.dbs = append(dlock.dbs, dbs...)
	dlock.mutex = make(map[string]rwlock.Mutex, 100)
}

type rwLock struct {
	dbs   []*sql.DB
	size  int
	m     sync.Mutex
	mutex map[string]rwlock.Mutex
}

func (rw *rwLock) allocation(name string, opts *rwlock.Options) rwlock.Mutex {
	rw.m.Lock()
	defer rw.m.Unlock()
	if rw.mutex[name] == nil {
		index := len(name) % rw.size
		rw.mutex[name] = &rwMysql{
			db:     rw.dbs[index],
			name:   name,
			opts:   opts,
			signal: make(chan struct{}, 1),
		}
	}
	return rw.mutex[name]
}

func Mutex(name string, opts ...rwlock.Option) rwlock.Mutex {
	ops := &rwlock.Options{}
	for _, o := range opts {
		o(ops)
	}
	return dlock.allocation(name, ops)
}

func RWMutex(name string, opts ...rwlock.Option) rwlock.RWMutex { // nolint
	return nil
}
