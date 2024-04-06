package mutex

import "os"
import "golang.org/x/sys/windows"

func getLock(file *os.File) error {
	handle := file.Fd()
	// 获取锁定文件的大小
	var fileSize int64
	fileStat, err := file.Stat()
	if err != nil {
		return err
	}
	fileSize = fileStat.Size()
	// 锁定整个文件
	overlapped := &windows.Overlapped{}
	err = windows.LockFileEx(windows.Handle(handle), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 0, uint32(fileSize), overlapped)
	if err != nil {
		return err
	}
	return nil
}

func releaseUnlock(file *os.File) error {
	handle := file.Fd()
	// 获取锁定文件的大小
	var fileSize int64
	fileStat, err := file.Stat()
	if err != nil {
		return err
	}
	fileSize = fileStat.Size()
	// 解锁整个文件
	overlapped := &windows.Overlapped{}
	err = windows.UnlockFileEx(windows.Handle(handle), 0, 0, uint32(fileSize), overlapped)
	if err != nil {
		return err
	}
	return nil
}
