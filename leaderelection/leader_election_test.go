package leaderelection

import (
	"context"
	"database/sql"
	"log"
	"testing"
	"time"

	"github.com/J-guanghua/rwlock/db"
	rwredis "github.com/J-guanghua/rwlock/redis"
	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
)

func TestMysqlElectionRunOrDie(_ *testing.T) {
	db2, err := sql.Open("mysql", "root:guanghua@tcp(192.168.43.152:3306)/sys?parseTime=true")
	if err != nil {
		panic(err)
	}
	db.Init(db2)
	ctx, _ := context.WithTimeout(context.TODO(), 1000*time.Second) // nolint
	MysqlRunOrDie(ctx, "guanghua", LeaderElectionConfig{
		OnStoppedLeading: func(identityID string) {
			log.Printf("我退出了,身份ID: %v", identityID)
		},
		OnNewLeader: func(identityID string) {
			log.Printf("我当选了,身份ID: %v", identityID)
		},
		OnStartedLeading: func(ctx context.Context) {
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
					log.Printf("我在的..............")
				}
			}
		},
	})
}

func TestRedisElectionRunOrDie(_ *testing.T) {
	rwredis.Init(&redis.Options{
		Addr:         "192.168.43.152:6379",
		PoolSize:     20,               // 连接池大小
		MinIdleConns: 10,               // 最小空闲连接数
		MaxConnAge:   time.Hour,        // 连接的最大生命周期
		PoolTimeout:  30 * time.Second, // 获取连接的超时时间
		IdleTimeout:  10 * time.Minute, // 连接的最大空闲时间
	})
	ctx, _ := context.WithTimeout(context.TODO(), 1000*time.Second) // nolint
	RedisRunOrDie(ctx, "guanghua", LeaderElectionConfig{
		OnStoppedLeading: func(identityID string) {
			log.Printf("我退出了,身份ID: %v", identityID)
			// 重新参与选举
			// TestRedisElectionRunOrDie(t)
		},
		OnNewLeader: func(identityID string) {
			log.Printf("我当选了,身份ID: %v", identityID)
		},
		OnStartedLeading: func(ctx context.Context) {
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
					log.Printf("我在的..................")
				}
			}
		},
	})
}
