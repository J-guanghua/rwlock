package leaderelection

import (
	"context"
	"time"

	"github.com/J-guanghua/rwlock"
	"github.com/J-guanghua/rwlock/db"
	"github.com/J-guanghua/rwlock/redis"
	"github.com/google/uuid"
)

// Leadership election configuration
type LeaderElectionConfig struct { // nolint
	IdentityID       string
	RetryPeriod      time.Duration
	RenewDeadline    time.Duration
	OnNewLeader      func(identityID string)
	OnStartedLeading func(ctx context.Context)
	OnStoppedLeading func(identityID string)
}

func (lec *LeaderElectionConfig) Init() {
	if lec.RetryPeriod == 0 {
		lec.RetryPeriod = 5 * time.Second
	}
	if lec.RenewDeadline == 0 {
		lec.RenewDeadline = 15 * time.Second
	}
	if lec.OnNewLeader == nil {
		lec.OnNewLeader = func(identityID string) {}
	}
	if lec.OnStartedLeading == nil {
		lec.OnStartedLeading = func(ctx context.Context) {}
	}
	if lec.OnStoppedLeading == nil {
		lec.OnStoppedLeading = func(identityID string) {}
	}
}

func (lec *LeaderElectionConfig) GetIdentityID() string {
	if lec.IdentityID == "" {
		lec.IdentityID = uuid.NewString()
	}
	return lec.IdentityID
}

// 自定义方案
// Mutex Lock 需要支持可重入
// Mutex 配置信息 选举IdentityID
func RunOrDie(ctx context.Context, mutex rwlock.Mutex, configuration LeaderElectionConfig) { // nolint
	configuration.Init()
	ctx2, cancel := context.WithCancel(ctx) // nolint
LeaderElection:
	ctx3, _ := context.WithTimeout(ctx, 200*time.Millisecond) // nolint
	if err := mutex.Lock(ctx3); err != nil {
		select {
		case <-ctx2.Done():
			return // nolint
		case <-time.After(configuration.RetryPeriod):
			goto LeaderElection
		}
	}

	// 当选 Leader
	configuration.OnNewLeader(configuration.GetIdentityID())
	go configuration.OnStartedLeading(ctx2)
	for { // nolint
		select { // nolint
		case <-time.After(configuration.RenewDeadline):
			if err := mutex.Lock(ctx2); err != nil {
				cancel()
				configuration.OnStoppedLeading(configuration.GetIdentityID())
				return
			}
		case <-ctx2.Done():
			configuration.OnStoppedLeading(configuration.GetIdentityID())
			_ = mutex.Unlock(ctx2)
			return
		}
	}
}

// redis实现选举机制
func RedisRunOrDie(ctx context.Context, name string, configuration LeaderElectionConfig) {
	configuration.Init()
	ctx2, cancel := context.WithCancel(ctx)
	mutex := redis.Mutex(name, rwlock.WithTries(2),
		rwlock.WithValue(configuration.GetIdentityID()),
		rwlock.WithExpiry(configuration.RenewDeadline+2*time.Second),
		rwlock.WithOnRenewal(func(renewal *rwlock.Renewal) {
			if renewal.Err != nil || !renewal.Result {
				defer cancel()
				renewal.Cancel()
			}
		}),
	)

LeaderElection:
	if err := mutex.Lock(ctx2); err != nil {
		select {
		case <-ctx2.Done():
			return // nolint
		case <-time.After(configuration.RetryPeriod):
			goto LeaderElection
		}
	}

	// 当选 Leader
	defer cancel()
	defer configuration.OnStoppedLeading(configuration.GetIdentityID())
	configuration.OnNewLeader(configuration.GetIdentityID())
	go configuration.OnStartedLeading(ctx2)
	<-ctx2.Done()
	_ = mutex.Unlock(ctx2)
}

// mysql实现选举机制
func MysqlRunOrDie(ctx context.Context, name string, configuration LeaderElectionConfig) {
	configuration.Init()
	ctx2, cancel := context.WithCancel(ctx)
	defer cancel()
	mutex := db.Mutex(name, rwlock.WithTries(2))
LeaderElection:
	if err := mutex.Lock(ctx2); err != nil {
		select {
		case <-ctx2.Done():
			return // nolint
		case <-time.After(configuration.RetryPeriod):
			goto LeaderElection
		}
	}

	// 当选 Leader
	configuration.OnNewLeader(configuration.GetIdentityID())
	go configuration.OnStartedLeading(ctx2)
	for { // nolint
		select { // nolint
		case <-time.After(configuration.RenewDeadline):
			if err := mutex.Lock(ctx2); err != nil {
				configuration.OnStoppedLeading(configuration.GetIdentityID())
				return
			}
		case <-ctx2.Done():
			configuration.OnStoppedLeading(configuration.GetIdentityID())
			_ = mutex.Unlock(ctx2)
			return
		}
	}
}
