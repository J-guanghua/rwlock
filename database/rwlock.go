package database

import (
	"database/sql"
	"github.com/J-guanghua/rwlock"
	"sync"
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
	dlock.mutex = map[string]rwlock.Mutex{}
}

type rwLock struct {
	dbs   []*sql.DB
	size  int
	m     sync.Mutex
	mutex map[string]rwlock.Mutex
}

func (dlock *rwLock) allocation(name string, opts *rwlock.Options) rwlock.Mutex {
	dlock.m.Lock()
	defer dlock.m.Unlock()
	if dlock.mutex[name] == nil {
		index := len(name) % dlock.size
		dlock.mutex[name] = &rwMysql{
			db:     dlock.dbs[index],
			name:   name,
			ops:    opts,
			signal: make(chan struct{}, 1),
		}
	}
	return dlock.mutex[name]
}

func NewLock(name string, opts ...rwlock.Option) rwlock.Mutex {
	ops := &rwlock.Options{}
	for _, o := range opts {
		o(ops)
	}
	return dlock.allocation(name, ops)
}

func NewRWLock(name string, opts ...rwlock.Option) rwlock.RWMutex {
	return nil
}
