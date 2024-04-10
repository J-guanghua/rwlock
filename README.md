# rwlock
分布式缓存方案


# go-cache

## Installation

To install go-cache, simply run:

    go get github.com/J-guanghua/rwlock

To compile it from source:

    cd $GOPATH/src/github.com/J-guanghua/rwlock
    go get -u -v
    go build && go test -v
### New cache interface
```go
    import (
        "github.com/J-guanghua/rwlock"
        "github.com/J-guanghua/rwlock/file"
        "github.com/J-guanghua/rwlock/redis"
        "github.com/J-guanghua/rwlock/database"
    )

    // init file lock
	// 压测 100万并发 左右
    Init("./tmp")

	mutex := file.NewLock("test-1")
    mutex := NewLock(name)
    defer mutex.Unlock(ctx)
    if err := mutex.Lock(ctx); err != nil {
        panic(err)
    }

    // init redis lock
	// 支持高可用，压测 100万并发 左右
	redis.Init(&redis.Options{
        Addr:         "127.0.0.1:6379",
        PoolSize:     20,               // 连接池大小
        MinIdleConns: 10,               // 最小空闲连接数
        MaxConnAge:   time.Hour,        // 连接的最大生命周期
        PoolTimeout:  30 * time.Second, // 获取连接的超时时间
        IdleTimeout:  10 * time.Minute, // 连接的最大空闲时间
    })
    
    mutex := redis.NewLock("test-1")
    mutex := NewLock(name)
    defer mutex.Unlock(ctx)
    if err := mutex.Lock(ctx); err != nil {
        panic(err)
    }
	
    // init db lock
	// 支持高可用，并发压测 5000 左右
    db, err := sql.Open("mysql", "root:guanghua@tcp(192.168.43.152:3306)/sys?parseTime=true")
    if err != nil {
        panic(err)
    }

    database.Init(db)
	mutex := database.NewLock("test-1")
    mutex := NewLock(name)
    defer mutex.Unlock(ctx)
    if err := mutex.Lock(ctx); err != nil {
        panic(err)
    }
        
```
### reference
```go

```
