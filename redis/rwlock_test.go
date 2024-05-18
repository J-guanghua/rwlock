package redis

import (
	"context"
	"fmt"
	"github.com/J-guanghua/rwlock"
	"github.com/go-redis/redis/v8"
	"log"
	"sync"
	"testing"
	"time"
)

func init() {

	Init(&redis.Options{
		Addr:         "192.168.43.152:6379",
		PoolSize:     20,               // 连接池大小
		MinIdleConns: 10,               // 最小空闲连接数
		MaxConnAge:   time.Hour,        // 连接的最大生命周期
		PoolTimeout:  30 * time.Second, // 获取连接的超时时间
		IdleTimeout:  10 * time.Minute, // 连接的最大空闲时间
	},
	)
}

func Test_RWLock_WaitGroup(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		name := fmt.Sprintf("group-%v", i)
		ctx, _ := context.WithTimeout(context.Background(), 25*time.Second)
		go func(ctx context.Context, name string, group *sync.WaitGroup) {
			var num int
			var wg2 sync.WaitGroup
			defer group.Done()
			for i := 0; i < 1000; i++ {
				wg2.Add(1)
				go func(name string) {
					defer wg2.Done()
					mutex := Mutex(name)
					if err := mutex.Lock(ctx); err != nil {
						panic(err)
					}
					num++
					err := mutex.Unlock(ctx)
					if err != nil {
						panic(err)
					}
				}(name)
			}
			wg2.Wait()
			log.Printf("%s,并发执行 %v 次,结果 %v ", name, 1000, num)
		}(ctx, name, &wg)
	}
	wg.Wait()
}

type Account struct {
	m        rwlock.Mutex
	balance  float64
	withhold float64
}

func (a *Account) alteration(ctx context.Context, value float64) error {
	if err := a.m.Lock(ctx); err != nil {
		return err
	}
	defer a.m.Unlock(ctx)
	a.balance -= value
	a.withhold += value
	return nil
}

func TestWaitGroupAccount(t *testing.T) {
	var wg sync.WaitGroup
	account := &Account{balance: 100002, m: Mutex("guanghua-2")}
	ctx, _ := context.WithTimeout(context.Background(), 100*time.Second)
	for i := 0; i < 100000; i++ {
		wg.Add(1)
		go func(acc *Account) {
			defer wg.Done()
			if err := account.alteration(ctx, 1); err != nil {
				log.Printf("失败:账户余额:%v", acc.balance)
				return
			}
		}(account)
	}
	wg.Wait()
	log.Printf("账户余额:%v,并发 100000,剩余 %v,", 100002, account.balance)
}
