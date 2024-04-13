package rwlock

import (
	"context"
	"github.com/google/uuid"
	"log"
	"time"
)

type LeaderElectionConfig struct {
	identityID       string
	Logger           *log.Logger
	RetryPeriod      time.Duration
	RenewDeadline    time.Duration
	OnNewLeader      func(currentId string)
	OnStartedLeading func(ctx context.Context)
	OnStoppedLeading func(quitId string)
}

func (config *LeaderElectionConfig) Init() {
	if config.OnStartedLeading == nil {
		config.OnStartedLeading = func(ctx context.Context) {}
	}
	if config.OnStartedLeading == nil {
		config.OnStartedLeading = func(ctx context.Context) {}
	}
	if config.Logger == nil {
		config.Logger = log.Default()
	}
	if config.RetryPeriod == 0 {
		config.RetryPeriod = 5 * time.Second
	}
	if config.OnNewLeader == nil {
		config.OnNewLeader = func(currentId string) {}
	}
	if config.GetIdentityID() == "" {
		config.identityID = uuid.NewString()
	}
}

func (config *LeaderElectionConfig) GetIdentityID() string {
	return config.identityID
}
