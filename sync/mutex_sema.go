// Copyright 2015 Aleksandr Demakin. All rights reserved.

package sync

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/avdva/go-ipc/internal/allocator"
	"github.com/avdva/go-ipc/internal/helper"
	"github.com/avdva/go-ipc/mmf"
	"github.com/avdva/go-ipc/shm"
)

// all implementations must satisfy IPCLocker interface.
var (
	_ IPCLocker = (*SemaMutex)(nil)
)

// SemaMutex is a semaphore-based mutex for unix.
type SemaMutex struct {
	s      *Semaphore
	region *mmf.MemoryRegion
	name   string
	lwm    *lwMutex
}

// NewSemaMutex creates a new semaphore-based mutex.
//	name - object name.
//	flag - flag is a combination of open flags from 'os' package.
//	perm - object's permission bits.
func NewSemaMutex(name string, flag int, perm os.FileMode) (*SemaMutex, error) {
	if err := ensureOpenFlags(flag); err != nil {
		return nil, err
	}
	region, created, err := helper.CreateWritableRegion(mutexSharedStateName(name, "s"), flag, perm, lwmStateSize)
	if err != nil {
		return nil, fmt.Errorf("creating shared state: %w", err)
	}
	s, err := NewSemaphore(name, flag, perm, 1)
	if err != nil {
		region.Close()
		if created {
			shm.DestroyMemoryObject(mutexSharedStateName(name, "s"))
		}
		return nil, fmt.Errorf("creating a semaphore: %w", err)
	}
	result := &SemaMutex{
		s:      s,
		region: region,
		name:   name,
		lwm:    newLightweightMutex(allocator.ByteSliceData(region.Data()), newSemaWaiter(s)),
	}
	if created {
		result.lwm.init()
	}
	return result, nil
}

// Lock locks the mutex. It panics on an error.
func (m *SemaMutex) Lock() {
	m.lwm.lock()
}

// LockTimeout tries to lock the locker, waiting for not more, than timeout.
func (m *SemaMutex) LockTimeout(timeout time.Duration) bool {
	return m.lwm.lockTimeout(timeout)
}

// TryLock makes one attempt to lock the mutex. It returns true on succeess and false otherwise.
func (m *SemaMutex) TryLock() bool {
	return m.lwm.tryLock()
}

// Unlock releases the mutex. It panics on an error, or if the mutex is not locked.
func (m *SemaMutex) Unlock() {
	m.lwm.unlock()
}

// Close closes shared state of the mutex.
func (m *SemaMutex) Close() error {
	e1, e2 := m.s.Close(), m.region.Close()
	if e1 != nil {
		return fmt.Errorf("closing semaphore: %w", e1)
	}
	if e2 != nil {
		return fmt.Errorf("closing shared state: %w", e2)
	}
	return nil
}

// Destroy closes the mutex and removes it permanently.
func (m *SemaMutex) Destroy() error {
	if err := m.Close(); err != nil {
		return fmt.Errorf("closing shared state: %w", err)
	}
	return DestroySemaMutex(m.name)
}

// DestroySemaMutex permanently removes mutex with the given name.
func DestroySemaMutex(name string) error {
	if err := shm.DestroyMemoryObject(mutexSharedStateName(name, "s")); err != nil {
		return fmt.Errorf("destroying shared state: %w", err)
	}
	if err := DestroySemaphore(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
