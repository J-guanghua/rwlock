package file

import (
	"context"
	"fmt"
	"github.com/J-guanghua/mutex"
	"log"
	"sync"
	"testing"
	"time"
)

func Test_fileLock_WaitGroup(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		name := fmt.Sprintf("group-%v", i)
		t.Run(name, func(t *testing.T) {
			ctx, _ := context.WithTimeout(context.Background(), 25*time.Second)
			defer wg.Done()
			WaitGroupTotal(ctx, name, 5000)
		})
	}
	wg.Wait()
}

func WaitGroupTotal(ctx context.Context, name string, total int) {
	var num int
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			mutex := NewMutex(ctx, name)
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
	wg.Wait()
	if total != num {
		log.Fatalf("%s,并发执行 %v 次,结果 %v ", name, total, num)
	}
	log.Printf("%s,并发执行 %v 次,结果 %v ", name, total, num)
}

func Test_fileLock_NewMutex(t *testing.T) {
	type fields struct {
		size      int
		m         sync.Mutex
		directory string
		mutex     map[string]*fMutex
	}
	type args struct {
		ctx  context.Context
		name string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   mutex.Mutex
	}{
		{
			name: "",
			fields: fields{
				directory: "./tmp2",
			},
			args: args{
				name: "lock-1",
			},
			want: &fMutex{},
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				name: "lock-2",
			},
			want: &fMutex{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flock := &fileLock{
				size:      tt.fields.size,
				m:         tt.fields.m,
				directory: tt.fields.directory,
				mutex:     tt.fields.mutex,
			}
			if got := flock.NewMutex(tt.args.ctx, tt.args.name); got == nil {
				t.Errorf("NewMutex() = %v, want %v", got, tt.want)
			}
		})
	}
}

type Account struct {
	mutex.Mutex
	balance  float64
	withhold float64
}

func (a *Account) alteration(ctx context.Context, value float64) error {
	if err := a.Lock(ctx); err != nil {
		return err
	}
	defer a.Unlock(ctx)
	a.balance -= value
	a.withhold += value
	return nil
}

func TestWaitGroupAccount(t *testing.T) {
	var wg sync.WaitGroup
	account := &Account{
		balance: 100002.90,
		Mutex:   flock.NewMutex(context.TODO(), "guanghua"),
	}
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
