// Copyright 2016 Aleksandr Demakin. All rights reserved.

//go:build windows || darwin
// +build windows darwin

package sync

import (
	"fmt"
	"os"
	"time"

	"github.com/avdva/go-ipc/internal/allocator"
	"github.com/avdva/go-ipc/internal/helper"
	"github.com/avdva/go-ipc/mmf"
	"github.com/avdva/go-ipc/shm"
)

type event struct {
	s      *Semaphore
	region *mmf.MemoryRegion
	name   string
	lwe    *lwEvent
}

func newEvent(name string, flag int, perm os.FileMode, initial bool) (*event, error) {
	if err := ensureOpenFlags(flag); err != nil {
		return nil, err
	}

	region, created, err := helper.CreateWritableRegion(eventName(name), flag, perm, lweStateSize)
	if err != nil {
		return nil, fmt.Errorf("creating shared state: %w", err)
	}
	s, err := NewSemaphore(name, flag, perm, 0)
	if err != nil {
		region.Close()
		if created {
			shm.DestroyMemoryObject(mutexSharedStateName(name, "s"))
		}
		return nil, fmt.Errorf("creating a semaphore: %w", err)
	}
	result := &event{
		lwe:    newLightweightEvent(allocator.ByteSliceData(region.Data()), newSemaWaiter(s)),
		name:   name,
		region: region,
		s:      s,
	}
	if created {
		result.lwe.init(initial)
	}
	return result, nil
}

func (e *event) set() {
	e.lwe.set()
}

func (e *event) wait() {
	e.waitTimeout(-1)
}

func (e *event) waitTimeout(timeout time.Duration) bool {
	return e.lwe.waitTimeout(timeout)
}

func (e *event) close() error {
	e1, e2 := e.s.Close(), e.region.Close()
	if e1 != nil {
		return fmt.Errorf("closing sema: %w", e1)
	}
	if e2 != nil {
		return fmt.Errorf("closing shared state: %w", e2)
	}
	return nil
}

func (e *event) destroy() error {
	if err := e.close(); err != nil {
		return fmt.Errorf("closing the event: %w", err)
	}
	return destroyEvent(e.name)
}

func destroyEvent(name string) error {
	e1, e2 := shm.DestroyMemoryObject(eventName(name)), destroySemaphore(name)
	if e1 != nil {
		return fmt.Errorf("destroying memory object: %w", e1)
	}
	if e2 != nil {
		return fmt.Errorf("destroying semaphore: %w", e2)
	}
	return nil
}
