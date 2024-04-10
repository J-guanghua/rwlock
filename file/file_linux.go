package file

import (
	"fmt"
	"os"
	"syscall"

	"github.com/J-guanghua/rwlock"
)

func acquireLock(file *os.File) error {
	// 尝试获取文件锁
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		fmt.Println("Failed to acquire lock:", err)
		return rwlock.ErrFail
	}
	return nil
}

func releaseLock(file *os.File) error {
	// 释放文件锁
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	if err != nil {
		fmt.Println("Failed to acquire lock:", err)
	}
	return err
}
