package redis

import (
	"context"
	"fmt"
	"github.com/J-guanghua/mutex"
	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"log"
	"sync"
	"testing"
	"time"
)

var rlock mutex.Lock

func init() {
	// 创建一个redis的客户端连接
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	// 创建redsync的客户端连接池
	pool := goredis.NewPool(client) // or, pool := redigo.NewPool(...)

	// 创建redsync实例
	redsync.New(pool)

	rlock = &redisLock{
		pool: []*redis.Client{
			redis.NewClient(
				&redis.Options{
					Addr:         "192.168.43.152:6379",
					PoolSize:     100,              // 连接池大小
					MinIdleConns: 10,               // 最小空闲连接数
					MaxConnAge:   time.Hour,        // 连接的最大生命周期
					PoolTimeout:  30 * time.Second, // 获取连接的超时时间
					IdleTimeout:  10 * time.Minute, // 连接的最大空闲时间
				},
			),
		},
	}
}

func Test_redisLock_WaitGroup(t *testing.T) {
	var wg sync.WaitGroup
	for i := 1; i < 100; i++ {
		wg.Add(1)
		name := fmt.Sprintf("group-%v", i)
		t.Run(name, func(t *testing.T) {
			ctx, _ := context.WithTimeout(context.Background(), 80*time.Second)
			go WaitGroupTotal(ctx, name, 10000, &wg)
		})
	}
	wg.Wait()
}

func WaitGroupTotal(ctx context.Context, name string, total int, group *sync.WaitGroup) {
	defer group.Done()
	var num int
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			relock := rlock.NewMutex(ctx, name,
				mutex.WithExpiry(2*time.Second),
				//mutex.WithValue(fmt.Sprintf("%v", i)),
				mutex.WithTouchf(func(ctx context.Context, name string) time.Duration {
					//log.Printf("Touchf:%v", name)
					return 8 * time.Second
				}),
			)
			if err := relock.Lock(ctx); err != nil {
				//log.Printf("Lock异常 : nane - %v,Gid: %v ,%v", name, mutex.GetGoroutineID(), err)
				return
			}
			num++
			err := relock.Unlock(ctx)
			if err != nil {
				log.Printf("Unlock异常 : name - %v,Gid: %v,%v ", name, mutex.GetGoroutineID(), err)
			}
		}(name)
	}
	wg.Wait()
	//if total != num {
	//	log.Printf("%s,并发执行 %v 次,结果 %v ", name, total, num)
	//}
	log.Printf("%s,并发执行 %v 次,结果 %v ", name, total, num)
}

type Account struct {
	mutex.Mutex
	money    float64
	withhold float64
}

func (a *Account) alteration(ctx context.Context, value float64) error {
	if err := a.Lock(ctx); err != nil {
		return err
	}
	//time.Sleep(2 * time.Second)
	defer a.Unlock(ctx)
	a.money -= value
	a.withhold += value
	return nil
}

type Order struct {
	name string
	mutex.Mutex
	money     float64
	inventory int
}

func (o *Order) placeAnOrder(ctx context.Context, account *Account) error {
	if err := o.Lock(ctx); err != nil {
		return err
	}
	defer o.Unlock(ctx)
	err := account.alteration(ctx, o.money)
	if err != nil {
		return err
	}
	o.inventory -= 1
	return nil
}

func Test_Order_WaitGroup(t *testing.T) {
	var wg sync.WaitGroup
	account := &Account{
		money: 1000002,
		Mutex: rlock.NewMutex(context.TODO(), "guanghua"),
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		name := fmt.Sprintf("order-%v", i)
		func(name string) {
			ctx, _ := context.WithTimeout(context.Background(), 80*time.Second)
			go WaitGroupPlaceAnOrder(ctx, &Order{
				name:      name,
				Mutex:     rlock.NewMutex(context.TODO(), name),
				money:     100,
				inventory: 1000,
			}, account, &wg)
		}(name)
	}
	wg.Wait()
}

func WaitGroupPlaceAnOrder(ctx context.Context, order *Order, account *Account, group *sync.WaitGroup) {
	var wg sync.WaitGroup
	defer group.Done()
	inventory := order.inventory
	for i := 0; i < inventory; i++ {
		wg.Add(1)
		go func(acc *Account) {
			defer wg.Done()
			if err := order.placeAnOrder(ctx, acc); err != nil {
				log.Printf("下单失败: nane - %v,库存: %v ,账户余额:%v", order.name, order.inventory, acc.money)
				return
			}
			money := acc.money
			log.Printf("下单成功: nane - %v,库存: %v ,账户余额:%v", order.name, order.inventory, money)
		}(account)
	}
	wg.Wait()
	log.Println(account.withhold)
}

func TestWaitGroupAccount(t *testing.T) {
	var wg sync.WaitGroup
	account := &Account{
		money: 802,
		Mutex: rlock.NewMutex(context.TODO(), "guanghua",
			mutex.WithExpiry(3*time.Second),
			//mutex.WithTouchf(func(ctx context.Context, name string) time.Duration {
			//	log.Printf("续期:%v;", name)
			//	return 15 * time.Second
			//}),
		),
	}
	ctx, _ := context.WithTimeout(context.Background(), 100*time.Second)
	for i := 0; i < 800; i++ {
		wg.Add(1)
		go func(acc *Account) {
			defer wg.Done()
			if err := account.alteration(ctx, 1); err != nil {
				log.Printf("失败:账户余额:%v", acc.money)
				return
			}
		}(account)
	}
	wg.Wait()
	log.Printf("账户余额:%v,并发 8000,剩余 %v,", 802, account.money)
}

func BenchmarkAccount(b *testing.B) {
	account := &Account{
		money: 100000002,
		Mutex: rlock.NewMutex(context.TODO(), "guanghua"),
	}
	ctx, _ := context.WithTimeout(context.Background(), 100*time.Second)
	for i := 0; i < b.N; i++ {
		if err := account.alteration(ctx, 1); err != nil {
			log.Printf("失败:账户余额:%v", account.money)
			return
		}
	}
}
