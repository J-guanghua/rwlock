package file

import (
	"fmt"
	"github.com/J-guanghua/rwlock"
	"os"
	"sync"
)

type rwLock struct {
	size      int
	mtx       sync.Mutex
	directory string
	mutex     map[string]*rwFile
}

func Init(filePath string) {
	flock.mtx.Lock()
	defer flock.mtx.Unlock()
	if filePath == "" {
		flock.directory = "./tmp"
	}
	flock.directory = filePath
	_, err := os.Stat(flock.directory)
	if os.IsNotExist(err) {
		err = os.Mkdir(flock.directory, 0666)
		if err != nil {
			panic(err)
		}
	}
	flock.mutex = make(map[string]*rwFile)
}

func (flock *rwLock) allocation(name string) rwlock.Mutex {
	flock.mtx.Lock()
	defer flock.mtx.Unlock()
	if flock.mutex[name] == nil {
		filepath := fmt.Sprintf("%s/%s.txt", flock.directory, name)
		file, err := os.OpenFile(filepath, os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			panic(err)
		}
		return &rwFile{
			name: name,
			file: file,
		}
	}
	return flock.mutex[name]
}

var flock rwLock

func Mutex(name string, opts ...rwlock.Option) rwlock.Mutex {
	return flock.allocation(name)
}
func RWMutex(name string, opts ...rwlock.Option) rwlock.RWMutex {
	return nil
}
