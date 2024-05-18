package leaderelection

import (
	"context"
	"time"

	"github.com/J-guanghua/rwlock"
	"github.com/J-guanghua/rwlock/database"
	"github.com/J-guanghua/rwlock/redis"
	"github.com/google/uuid"
)

// Leadership election configuration
type LeaderElectionConfig struct { // nolint
	identityID       string
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
	if lec.identityID == "" {
		lec.identityID = uuid.NewString()
	}
	return lec.identityID
}

// 自定义方案
// Mutex Lock 需要支持重入
// Mutex 配置信息 选举IdentityID,续约时长 可以通过 FromContext 获取
func LeaderElectionRunOrDie(ctx context.Context, mutex rwlock.Mutex, configuration LeaderElectionConfig) { // nolint
	configuration.Init()
	ctx2, cancel := context.WithCancel(ctx)
LeaderElection:
	ctx3, _ := context.WithTimeout(ctx, 200*time.Millisecond) // nolint
	if err := mutex.Lock(ctx3); err != nil {
		<-time.After(configuration.RetryPeriod)
		goto LeaderElection
	}

	// 当选 Leader
	configuration.OnNewLeader(configuration.GetIdentityID())
	go configuration.OnStartedLeading(ctx2)
	for { // nolint
		select { // nolint
		case <-time.After(configuration.RenewDeadline):
			if err := mutex.Lock(ctx2); err != nil {
				cancel()
				_ = mutex.Unlock(ctx2)
				configuration.OnStoppedLeading(configuration.GetIdentityID())
				return
			}
		}
	}
}

func RedisElectionRunOrDie(ctx context.Context, name string, configuration LeaderElectionConfig) {
	ctx2, cancel := context.WithCancel(ctx)
	mutex := redis.Mutex(name, rwlock.WithValue(configuration.GetIdentityID()),
		rwlock.WithExpiry(configuration.RenewDeadline+2*time.Second),
		rwlock.WithTouchf(func(touch *rwlock.Renewal) {
			if touch.Err != nil || !touch.Result {
				defer cancel()
				touch.Cancel()
			}
		}),
	)

LeaderElection:
	ctx3, _ := context.WithTimeout(ctx, 200*time.Millisecond) // nolint
	if err := mutex.Lock(ctx3); err != nil {
		<-time.After(configuration.RetryPeriod)
		goto LeaderElection
	}

	// 当选 Leader
	defer cancel()
	defer configuration.OnStoppedLeading(configuration.GetIdentityID())
	configuration.OnNewLeader(configuration.GetIdentityID())
	go configuration.OnStartedLeading(ctx2)
	<-ctx2.Done()
	_ = mutex.Unlock(ctx2)
}

func MysqlElectionRunOrDie(ctx context.Context, name string, configuration LeaderElectionConfig) {
	configuration.Init()
	ctx2, cancel := context.WithCancel(ctx)
	mutex := database.Mutex(name)
LeaderElection:
	ctx3, _ := context.WithTimeout(ctx, 200*time.Millisecond) // nolint
	if err := mutex.Lock(ctx3); err != nil {
		<-time.After(configuration.RetryPeriod)
		goto LeaderElection
	}

	// 当选 Leader
	configuration.OnNewLeader(configuration.GetIdentityID())
	go configuration.OnStartedLeading(ctx2)
	for { // nolint
		select { // nolint
		case <-time.After(configuration.RenewDeadline):
			if err := mutex.Lock(ctx2); err != nil {
				cancel()
				_ = mutex.Unlock(ctx2)
				configuration.OnStoppedLeading(configuration.GetIdentityID())
				return
			}
		}
	}
}
