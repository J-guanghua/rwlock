package database

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/J-guanghua/rwlock"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"sync"
	"testing"
	"time"
)

func init() {
	db2, err := sql.Open("mysql", "root:guanghua@tcp(192.168.43.152:3306)/sys?parseTime=true")
	if err != nil {
		panic(err)
	}
	Init(db2)

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
	account := &Account{balance: 10002, m: Mutex("guanghua")}
	ctx, _ := context.WithTimeout(context.Background(), 100*time.Second)
	for i := 0; i < 10000; i++ {
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

func TestLeaderElection(t *testing.T) {
	StartElection(context.TODO())
}

func StartElection(ctx context.Context) {
	LeaderElectionRunOrDie(ctx, "guanghua", rwlock.LeaderElectionConfig{
		OnStoppedLeading: func(identityID string) {
			log.Printf("我退出了,身份ID: %v", identityID)
			StartElection(ctx)
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
