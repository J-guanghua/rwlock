package file

import (
	"fmt"
	"io/fs"
	"os"
	"sync"

	"github.com/J-guanghua/rwlock"
)

type rwLock struct {
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
		if err = os.Mkdir(flock.directory, fs.FileMode(0o666)); err != nil {
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
		file, err := os.OpenFile(filepath, os.O_CREATE|os.O_RDWR, fs.FileMode(0o666))
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

func Mutex(name string, _ ...rwlock.Option) rwlock.Mutex {
	return flock.allocation(name)
}

func RWMutex(_ string, _ ...rwlock.Option) rwlock.RWMutex { // onlit
	return nil
}
