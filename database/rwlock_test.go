package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/J-guanghua/rwlock"
	_ "github.com/go-sql-driver/mysql"
)

func init() {
	db2, err := sql.Open("mysql", "root:guanghua@tcp(192.168.43.152:3306)/sys?parseTime=true")
	if err != nil {
		panic(err)
	}
	Init(db2)
}

func Test_RWLock_WaitGroup(_ *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		name := fmt.Sprintf("group-%v", i)
		ctx, _ := context.WithTimeout(context.Background(), 25*time.Second) // nolint
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
	a.balance -= value
	a.withhold += value
	return a.m.Unlock(ctx)
}

func TestWaitGroupAccount(_ *testing.T) {
	var wg sync.WaitGroup
	account := &Account{balance: 10002, m: Mutex("guanghua")}
	ctx, _ := context.WithTimeout(context.Background(), 100*time.Second) // nolint
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
