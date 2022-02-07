// Copyright 2016 Aleksandr Demakin. All rights reserved.

//go:build darwin || freebsd || linux
// +build darwin freebsd linux

package sync

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/avdva/go-ipc/v2/internal/common"
)

const (
	cSemUndo = 0x1000
)

type sembuf struct {
	semnum uint16
	semop  int16
	semflg int16
}

// semaphore is a sysV semaphore.
type semaphore struct {
	name string
	id   int
}

// newSemaphore creates a new sysV semaphore with the given name.
// It generates a key from the name, and then calls NewSemaphoreKey.
func newSemaphore(name string, flag int, perm os.FileMode, initial int) (*semaphore, error) {
	k, err := common.KeyForName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to generate a key for the name: %w", err)
	}
	result, err := newSemaphoreKey(uint64(k), flag, perm, initial)
	if err != nil {
		return nil, err
	}
	result.name = name
	return result, nil
}

// newSemaphoreKey creates a new sysV semaphore for the given key.
func newSemaphoreKey(key uint64, flag int, perm os.FileMode, initial int) (*semaphore, error) {
	var id int
	creator := func(create bool) error {
		var creatorErr error
		flags := int(perm)
		if create {
			flags |= common.IpcCreate | common.IpcExcl
		}
		id, creatorErr = semget(common.Key(key), 1, flags)
		return creatorErr
	}
	created, err := common.OpenOrCreate(creator, flag)
	if err != nil {
		return nil, fmt.Errorf("opening sysv semaphore: %w", err)
	}
	result := &semaphore{id: id}
	if created && initial > 0 {
		if err = result.add(initial); err != nil {
			result.Destroy()
			return nil, fmt.Errorf("adding initial semaphore value: %w", err)
		}
	}
	return result, nil
}

func (s *semaphore) signal(count int) {
	if err := s.add(count); err != nil {
		panic(err)
	}
}

func (s *semaphore) wait() {
	if err := s.add(-1); err != nil {
		panic(err)
	}
}

func (s *semaphore) waitTimeout(timeout time.Duration) bool {
	if timeout < 0 {
		s.wait()
		return true
	}
	return doSemaTimedWait(s.id, timeout)
}

func (s *semaphore) close() error {
	return nil
}

func (s *semaphore) Destroy() error {
	return removeSysVSemaByID(s.id, s.name)
}

func (s *semaphore) add(value int) error {
	return common.UninterruptedSyscall(func() error { return semAdd(s.id, value) })
}

// destroySemaphore permanently removes semaphore with the given name.
func destroySemaphore(name string) error {
	k, err := common.KeyForName(name)
	if err != nil {
		return fmt.Errorf("getting a key for the name: %w", err)
	}
	id, err := semget(k, 1, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("getting semaphore id: %w", err)
	}
	return removeSysVSemaByID(id, name)
}

func removeSysVSemaByID(id int, name string) error {
	err := semctl(id, 0, common.IpcRmid)
	if err == nil && len(name) > 0 {
		if err = os.Remove(common.TmpFilename(name)); os.IsNotExist(err) {
			err = nil
		} else if err != nil {
			err = fmt.Errorf("removing temporary file: %w", err)
		}
	} else if os.IsNotExist(err) {
		err = nil
	} else {
		err = fmt.Errorf("semctl: %w", err)
	}
	return err
}

func semAdd(id, value int) error {
	b := sembuf{semnum: 0, semop: int16(value), semflg: 0}
	return semop(id, []sembuf{b})
}
