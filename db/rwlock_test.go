package db

import (
	"context"
	"database/sql"
	"fmt"
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

func Test_RWLock_WaitGroup(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		name := fmt.Sprintf("group-%v", i)
		ctx, _ := context.WithTimeout(context.Background(), 25*time.Second) // nolint
		go func(ctx context.Context, name string, group *sync.WaitGroup) {
			var num int
			var wg2 sync.WaitGroup
			defer group.Done()
			for i := 0; i < 100; i++ {
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
			t.Logf("%s,并发执行 %v 次,结果 %v ", name, 100, num)
		}(ctx, name, &wg)
	}
	wg.Wait()
}

type account struct {
	m        rwlock.Mutex
	balance  float64
	withhold float64
}

func (a *account) alteration(ctx context.Context, value float64) error {
	if err := a.m.Lock(ctx); err != nil {
		return err
	}
	a.balance -= value
	a.withhold += value
	return a.m.Unlock(ctx)
}

func TestWaitGroupAccount(t *testing.T) {
	var wg sync.WaitGroup
	a := &account{balance: 102, m: Mutex("guanghua")}
	ctx, _ := context.WithTimeout(context.Background(), 100*time.Second) // nolint
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(acc *account) {
			defer wg.Done()
			if err := a.alteration(ctx, 1); err != nil {
				t.Logf("失败:账户余额:%v", acc.balance)
				return
			}
		}(a)
	}
	wg.Wait()
	t.Logf("账户余额:%v,并发 1000,剩余 %v,", 1002, a.balance)
}

func BenchmarkRWMutex(b *testing.B) {
	mutex := Mutex("test")
	for i := 0; i < b.N; i++ {
		if err := mutex.Lock(context.TODO()); err != nil {
			b.Error(err)
		}
		_ = mutex.Unlock(context.TODO())
	}
}
