// Copyright 2015 Aleksandr Demakin. All rights reserved.

package sync

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
)

func rwMutexCtor(name string, flag int, perm os.FileMode) (IPCLocker, error) {
	return NewRWMutex(name, flag, perm)
}

func rwRMutexCtor(name string, flag int, perm os.FileMode) (IPCLocker, error) {
	locker, err := NewRWMutex(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return locker.RLocker(), nil
}

func rwMutexDtor(name string) error {
	return DestroyRWMutex(name)
}

func TestRWMutexOpenMode(t *testing.T) {
	testLockerOpenMode(t, rwMutexCtor, rwMutexDtor)
}

func TestRWMutexOpenMode2(t *testing.T) {
	testLockerOpenMode2(t, rwMutexCtor, rwMutexDtor)
}

func TestRWMutexOpenMode3(t *testing.T) {
	testLockerOpenMode3(t, rwMutexCtor, rwMutexDtor)
}

func TestRWMutexOpenMode4(t *testing.T) {
	testLockerOpenMode4(t, rwMutexCtor, rwMutexDtor)
}

func TestRWMutexOpenMode5(t *testing.T) {
	testLockerOpenMode5(t, rwMutexCtor, rwMutexDtor)
}

func TestRWMutexLock(t *testing.T) {
	testLockerLock(t, rwMutexCtor, rwMutexDtor)
}

func TestRWMutexMemory(t *testing.T) {
	testLockerMemory(t, "rw", false, rwMutexCtor, rwMutexDtor)
}

func TestRWMutexMemory2(t *testing.T) {
	testLockerMemory(t, "rw", true, rwMutexCtor, rwMutexDtor)
}

func TestRWMutexValueInc(t *testing.T) {
	testLockerValueInc(t, "rw", rwMutexCtor, rwMutexDtor)
}

func TestRWMutexPanicsOnDoubleUnlock(t *testing.T) {
	testLockerTwiceUnlock(t, rwMutexCtor, rwMutexDtor)
}

func TestRWMutexPanicsOnDoubleRUnlock(t *testing.T) {
	testLockerTwiceUnlock(t, rwRMutexCtor, rwMutexDtor)
}

func ExampleRWMutex() {
	const (
		parallel = 20
	)
	err := DestroyRWMutex("rw")
	if err != nil {
		panic(err)
	}
	m, err := NewRWMutex("rw", os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		panic(err)
	}
	// we create a shared array of constantly increasing ints for reading and wriring.
	sharedData := make([]int, 8*1024)
	for i := range sharedData {
		sharedData[i] = i
	}
	var wg sync.WaitGroup
	wg.Add(parallel)
	// writers will update the data.
	for i := 0; i < parallel/2; i++ {
		go func() {
			defer wg.Done()
			start := rand.Intn(1024)
			m.Lock()
			defer m.Unlock()
			for i := range sharedData {
				sharedData[i] = 0
			}
			for i := range sharedData {
				sharedData[i] = i + start
			}
		}()
		go func() {
			defer wg.Done()
			m.Lock()
			defer m.Unlock()
			for i := 1; i < len(sharedData); i++ {
				if sharedData[i] != sharedData[i-1]+1 {
					panic("bad data")
				}
			}
		}()
	}
	wg.Wait()
	fmt.Println("done")
	// Output:
	// done
}
