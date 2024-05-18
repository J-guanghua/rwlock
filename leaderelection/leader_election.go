package leaderelection

import (
	"context"
	"github.com/J-guanghua/rwlock"
	"github.com/J-guanghua/rwlock/database"
	"github.com/J-guanghua/rwlock/redis"
	"github.com/google/uuid"
	"time"
)

// 领导人选举配置
type LeaderElectionConfig struct {
	identityID       string
	RetryPeriod      time.Duration
	RenewDeadline    time.Duration
	OnNewLeader      func(identityID string)
	OnStartedLeading func(ctx context.Context)
	OnStoppedLeading func(identityID string)
}

func (config *LeaderElectionConfig) Init() {
	if config.RetryPeriod == 0 {
		config.RetryPeriod = 5 * time.Second
	}
	if config.RenewDeadline == 0 {
		config.RenewDeadline = 15 * time.Second
	}
	if config.OnNewLeader == nil {
		config.OnNewLeader = func(identityID string) {}
	}
	if config.OnStartedLeading == nil {
		config.OnStartedLeading = func(ctx context.Context) {}
	}
	if config.OnStoppedLeading == nil {
		config.OnStoppedLeading = func(identityID string) {}
	}
}

func (config *LeaderElectionConfig) GetIdentityID() string {
	if config.identityID == "" {
		config.identityID = uuid.NewString()
	}
	return config.identityID
}

// 自定义方案
// Mutex Lock 需要支持重入
// Mutex 配置信息 选举IdentityID,续约时长 可以通过 FromContext 获取
func LeaderElectionRunOrDie(ctx context.Context, mutex rwlock.Mutex, config LeaderElectionConfig) {
	config.Init()
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

func RedisElectionRunOrDie(ctx context.Context, name string, config LeaderElectionConfig) {
	ctx2, cancel := context.WithCancel(ctx)
	mutex := redis.Mutex(name, rwlock.WithValue(config.GetIdentityID()),
		rwlock.WithExpiry(config.RenewDeadline+2*time.Second),
		rwlock.WithTouchf(func(touch *rwlock.Renewal) {
			if touch.Err != nil || touch.Result == false {
				defer cancel()
				touch.Cancel()
			}
		}),
	)

LeaderElection:
	ctx3, _ := context.WithTimeout(ctx, 200*time.Millisecond)
	if err := mutex.Lock(ctx3); err != nil {
		<-time.After(config.RetryPeriod)
		goto LeaderElection
	}

	// 当选 Leader
	defer cancel()
	defer mutex.Unlock(ctx2)
	defer config.OnStoppedLeading(config.GetIdentityID())
	config.OnNewLeader(config.GetIdentityID())
	go config.OnStartedLeading(ctx2)
	select {
	case <-ctx2.Done():
	}
}

func MysqlElectionRunOrDie(ctx context.Context, name string, config LeaderElectionConfig) {
	config.Init()
	ctx2, cancel := context.WithCancel(ctx)
	mutex := database.Mutex(name)
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
