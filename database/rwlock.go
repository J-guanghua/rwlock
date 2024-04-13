package database

import (
	"context"
	"database/sql"
	"github.com/J-guanghua/rwlock"
	"sync"
	"time"
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

func Mutex(name string, opts ...rwlock.Option) rwlock.Mutex {
	ops := &rwlock.Options{}
	for _, o := range opts {
		o(ops)
	}
	return dlock.allocation(name, ops)
}

func RWMutex(name string, opts ...rwlock.Option) rwlock.RWMutex {
	return nil
}

func LeaderElectionRunOrDie(ctx context.Context, name string, config rwlock.LeaderElectionConfig) {
	config.Init()
	mutex := Mutex(name)
	ctx2, cancel := context.WithCancel(ctx)
LeaderElection:
	ctx3, _ := context.WithTimeout(ctx, 200*time.Millisecond)
	if err := mutex.Lock(ctx3); err != nil {
		<-time.After(config.RetryPeriod)
		goto LeaderElection
	}

	// 当选 Leader
	config.OnNewLeader(config.GetIdentityID())
	go config.OnStartedLeading(ctx2)
	for {
		select {
		case <-time.After(config.RenewDeadline):
			if err := mutex.Lock(ctx2); err != nil {
				cancel()
				mutex.Unlock(ctx2)
				config.OnStoppedLeading(config.GetIdentityID())
				return
			}
		}
	}
}
