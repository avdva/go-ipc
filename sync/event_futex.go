// Copyright 2016 Aleksandr Demakin. All rights reserved.

// +build freebsd linux

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
	name   string
	region *mmf.MemoryRegion
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
	state := allocator.ByteSliceData(region.Data())
	result := &event{
		lwe:    newLightweightEvent(state, &futex{ptr: state}),
		name:   name,
		region: region,
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
	return e.region.Close()
}

func (e *event) destroy() error {
	if err := e.close(); err != nil {
		return fmt.Errorf("closing shm region: %w", err)
	}
	return destroyEvent(e.name)
}

func destroyEvent(name string) error {
	err := shm.DestroyMemoryObject(eventName(name))
	if err != nil {
		return fmt.Errorf("destroying memory object: %w", err)
	}
	return nil
}
