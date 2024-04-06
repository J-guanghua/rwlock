package file

import (
	"context"
	"errors"
	"fmt"
	"github.com/J-guanghua/mutex"
	"os"
	"sync"
	"time"
)

type fileLock struct {
	size      int
	m         sync.Mutex
	directory string
	mutex     map[string]*fMutex
}

func (flock *fileLock) allocation(name string) mutex.Mutex {
	flock.m.Lock()
	defer flock.m.Unlock()
	if flock.mutex == nil {
		flock.mutex = map[string]*fMutex{}
	} else if mutex := flock.mutex[name]; mutex != nil {
		return mutex
	}
	return flock.loadMutex(name, &fMutex{
		name: name,
	})
}

func (flock *fileLock) loadMutex(name string, file2 *fMutex) mutex.Mutex {
	if flock.directory == "" {
		flock.directory = "./tmp"
	}
	_, err := os.Stat(flock.directory)
	if os.IsNotExist(err) {
		err = os.Mkdir(flock.directory, 0666)
		if err != nil {
			panic(err)
		}
	}
	filename := fmt.Sprintf("%s/%s.txt", flock.directory, file2.name)
	file2.file, err = os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	flock.mutex[name] = file2
	return file2
}

var flock fileLock

func NewMutex(ctx context.Context, name string) mutex.Mutex {
	return flock.allocation(name)
}

func (flock *fileLock) NewMutex(ctx context.Context, name string) mutex.Mutex {
	return flock.allocation(name)
}

type fMutex struct {
	file *os.File
	name string
	m    sync.Mutex
}

func (file *fMutex) Lock(ctx context.Context) (err error) {
	err = file.acquireLock(ctx)
	if !errors.Is(err, mutex.ErrFail) {
		return err
	} else if err != nil {
	LoopLock:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1000 * time.Millisecond):
			err := file.acquireLock(ctx)
			if errors.Is(err, mutex.ErrFail) {
				goto LoopLock
			} else if err != nil {
				return err
			}
		}
	}
	return nil
}

func (file *fMutex) Unlock(ctx context.Context) error {
	return file.releaseLock(ctx)
}

// 获取文件句柄
func (file *fMutex) acquireLock(ctx context.Context) error {
	file.m.Lock()
	//defer fMutex.m.Unlock()
	return acquireLock(file.file)
}

// 释放文件锁
func (file *fMutex) releaseLock(ctx context.Context) error {
	//fMutex.m.Lock()
	defer file.m.Unlock()
	return releaseLock(file.file)
}
