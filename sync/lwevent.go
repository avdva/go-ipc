// Copyright 2016 Aleksandr Demakin. All rights reserved.

package sync

import (
	"math"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/avdva/go-ipc/v2/internal/common"
)

const (
	lweStateSize = 4
)

// lwEvent is a lightweight event implementation operating on a uint32 memory cell.
// it tries to minimize amount of syscalls.
// actual wait/wake must be implemented by a waitWaker object.
// state is a shared variable, that contains event state:
//	the highest bit is a signal bit
//	all other bits define the number of waiters.
type lwEvent[wwImpl waitWaker] struct {
	state *int32
	ww    wwImpl
}

func newLightweightEvent[wwImpl waitWaker](state unsafe.Pointer, ww wwImpl) *lwEvent[wwImpl] {
	return &lwEvent[wwImpl]{state: (*int32)(state), ww: ww}
}

func (e *lwEvent[wwImpl]) init(set bool) {
	val := int32(0)
	if set {
		val = math.MinInt32
	}
	*e.state = val
}

func (e *lwEvent[wwImpl]) set() {
	var old int32
	for {
		old = atomic.LoadInt32(e.state)
		if old < 0 {
			return
		}
		new := old | math.MinInt32
		if atomic.CompareAndSwapInt32(e.state, old, new) {
			break
		}
	}
	if old > 0 {
		e.ww.wake(1)
	}
}

func (e *lwEvent[wwImpl]) obtainOrChange(inc int32) (new int32, obtained bool) {
	for {
		old := atomic.LoadInt32(e.state)
		new = old
		if old < 0 { // reset 'set' bit
			new = old & ^math.MinInt32
		} else { // change the value
			if inc == 0 {
				return
			}
			new = old + inc
		}
		if atomic.CompareAndSwapInt32(e.state, old, new) {
			if old < 0 { // bit was set and we reset it. success. otherwise, we changed the value.
				obtained = true
			}
			return
		}
	}
}

func (e *lwEvent[wwImpl]) waitTimeout(timeout time.Duration) bool {
	// first, we are trying to catch the event, or add us as a waiter.
	new, obtained := e.obtainOrChange(1)
	if obtained {
		return true
	}
	// in the loop we wait for the value to change and then observe new value:
	//	if it is still not set, wait again
	//	otherwise, try to obtain the event.
	for {
		if err := e.ww.wait(new, timeout); err != nil {
			if common.IsTimeoutErr(err) {
				_, obtained = e.obtainOrChange(-1)
				return obtained
			}
		}
		new, obtained = e.obtainOrChange(0)
		if obtained {
			return true
		}
	}
}
