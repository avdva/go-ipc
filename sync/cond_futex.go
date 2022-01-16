// Copyright 2016 Aleksandr Demakin. All rights reserved.

//+build freebsd linux

package sync

import (
	"fmt"
	"os"
	"time"

	"github.com/avdva/go-ipc/internal/allocator"
	"github.com/avdva/go-ipc/internal/common"
	"github.com/avdva/go-ipc/internal/helper"
	"github.com/avdva/go-ipc/mmf"
	"github.com/avdva/go-ipc/shm"
)

// cond is a futex-based convar.
type cond struct {
	L      IPCLocker
	name   string
	region *mmf.MemoryRegion
	ftx    *futex
}

func newCond(name string, flag int, perm os.FileMode, l IPCLocker) (*cond, error) {
	if err := ensureOpenFlags(flag); err != nil {
		return nil, err
	}

	region, _, err := helper.CreateWritableRegion(condSharedStateName(name), flag, perm, lwmStateSize)
	if err != nil {
		return nil, fmt.Errorf("creating shared state: %w", err)
	}

	result := &cond{
		L:      l,
		name:   name,
		ftx:    &futex{(allocator.ByteSliceData(region.Data()))},
		region: region,
	}

	return result, nil
}

func (c *cond) signal() {
	c.ftx.add(1)
	_, err := c.ftx.wake(1)
	if err != nil {
		panic(err)
	}
}

func (c *cond) broadcast() {
	c.ftx.add(1)
	_, err := c.ftx.wakeAll()
	if err != nil {
		panic(err)
	}
}

func (c *cond) wait() {
	seq := *c.ftx.addr()
	c.L.Unlock()
	if err := c.ftx.wait(seq, time.Duration(-1)); err != nil {
		panic(err)
	}
	c.L.Lock()
}

func (c *cond) waitTimeout(timeout time.Duration) bool {
	seq := *c.ftx.addr()
	var success bool
	c.L.Unlock()
	if err := c.ftx.wait(seq, timeout); err == nil {
		success = true
	} else if !common.IsTimeoutErr(err) {
		panic(err)
	}
	c.L.Lock()
	return success
}

func (c *cond) close() error {
	if err := c.region.Close(); err != nil {
		return fmt.Errorf("closing waiters list memory region: %w", err)
	}
	return nil
}

func (c *cond) destroy() error {
	var result error
	if err := c.close(); err != nil {
		result = fmt.Errorf("closing: %w", err)
	}
	if err := shm.DestroyMemoryObject(condSharedStateName(c.name)); err != nil {
		result = fmt.Errorf("closing waiters list memory object: %w", err)
	}
	return result
}

func destroyCond(name string) error {
	return shm.DestroyMemoryObject(condSharedStateName(name))
}

func condSharedStateName(name string) string {
	return name + ".st"
}
