package rwlock

import (
	"context"
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
func LeaderElectionRunOrDie(ctx context.Context, custom Mutex, config LeaderElectionConfig) {
	config.Init()
	ctx2, cancel := context.WithCancel(WithContext(ctx, &Options{
		Value:  config.GetIdentityID(),
		Expiry: config.RenewDeadline,
	}))

	// 周期进行重试抢锁
LeaderElection:
	if err := custom.Lock(ctx2); err != nil {
		<-time.After(config.RetryPeriod)
		goto LeaderElection
	}

	// 当选 Leader
	config.OnNewLeader(config.GetIdentityID())
	go config.OnStartedLeading(ctx2)
	for {
		select {
		case <-time.After(config.RenewDeadline - 2*time.Second):
			// 询问是否还持有 Leader身份,并续约时长
			if err := custom.Lock(ctx2); err != nil {
				// 退出
				cancel()
				custom.Unlock(ctx2)
				config.OnStoppedLeading(config.GetIdentityID())
				return
			}
		}
	}
}
