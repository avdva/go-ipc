// Copyright 2015 Aleksandr Demakin. All rights reserved.

package shm

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Shared memory on Windows is emulated via regular files
// like it is done in boost c++ library.
type memoryObject struct {
	file *os.File
}

func newMemoryObject(name string, flag int, perm os.FileMode) (impl *memoryObject, err error) {
	path, err := shmName(name)
	if err != nil {
		return nil, fmt.Errorf("shm name: %w", err)
	}
	file, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	return &memoryObject{file}, nil
}

func (obj *memoryObject) Destroy() error {
	if int(obj.Fd()) >= 0 {
		if err := obj.Close(); err != nil {
			return fmt.Errorf("closing file: %w", err)
		}
	}
	return DestroyMemoryObject(obj.Name())
}

func (obj *memoryObject) Name() string {
	return filepath.Base(obj.file.Name())
}

func (obj *memoryObject) Close() error {
	runtime.SetFinalizer(obj, nil)
	return obj.file.Close()
}

func (obj *memoryObject) Truncate(size int64) error {
	return obj.file.Truncate(size)
}

func (obj *memoryObject) Size() int64 {
	fileInfo, err := obj.file.Stat()
	if err != nil {
		return 0
	}
	return fileInfo.Size()
}

func (obj *memoryObject) Fd() uintptr {
	return obj.file.Fd()
}

func destroyMemoryObject(name string) error {
	path, err := shmName(name)
	if err != nil {
		return fmt.Errorf("shm name: %w", err)
	}
	if err = os.Remove(path); os.IsNotExist(err) {
		err = nil
	} else {
		err = fmt.Errorf("removing file: %w", err)
	}
	return err
}

func shmName(name string) (string, error) {
	path, err := sharedDirName()
	if err != nil {
		return "", fmt.Errorf("getting tmp directory name: %w", err)
	}
	return path + "/" + name, nil
}

func sharedDirName() (string, error) {
	rootPath := os.TempDir() + "/go-ipc"
	if err := os.Mkdir(rootPath, 0o644); err != nil && !os.IsExist(err) {
		return "", err
	}
	return rootPath, nil
}
