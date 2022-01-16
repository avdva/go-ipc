// Copyright 2015 Aleksandr Demakin. All rights reserved.

// +build darwin freebsd linux

package mmf

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/avdva/go-ipc/internal/allocator"

	"golang.org/x/sys/unix"
)

func init() {
	mmapOffsetMultiple = int64(os.Getpagesize())
}

type memoryRegion struct {
	data       []byte
	size       int
	pageOffset int64
}

func newMemoryRegion(obj Mappable, flag int, offset int64, size int) (*memoryRegion, error) {
	prot, flags, err := memProtAndFlagsFromMode(flag)
	if err != nil {
		return nil, fmt.Errorf("checking memory region flags: %w", err)
	}
	if size, err = checkMmapSize(obj, size); err != nil {
		return nil, fmt.Errorf("checking size: %w", err)
	}
	calculatedSize, err := fileSizeFromFd(obj)
	if err != nil {
		return nil, fmt.Errorf("checking file size: %w", err)
	}
	// we need this check on unix, because you can actually mmap more bytes,
	// then the size of the object, which can cause unexpected problems.
	if calculatedSize > 0 && int64(size)+offset > calculatedSize {
		return nil, fmt.Errorf("invalid mapping length")
	}
	pageOffset := calcMmapOffsetFixup(offset)
	var data []byte
	if data, err = unix.Mmap(int(obj.Fd()), offset-pageOffset, size+int(pageOffset), prot, flags); err != nil {
		return nil, fmt.Errorf("mmap failed: %w", err)
	}
	return &memoryRegion{data: data, size: size, pageOffset: pageOffset}, nil
}

func (region *memoryRegion) Close() error {
	if region.data == nil {
		err := unix.Munmap(region.data)
		*region = memoryRegion{}
		if err != nil {
			return fmt.Errorf("munmap failed: %w", err)
		}
	}
	return nil
}

func (region *memoryRegion) Data() []byte {
	return region.data[region.pageOffset:]
}

func (region *memoryRegion) Flush(async bool) error {
	flag := unix.MS_SYNC
	if async {
		flag = unix.MS_ASYNC
	}
	if err := msync(region.data, flag); err != nil {
		return fmt.Errorf("mync failed: %w", err)
	}
	return nil
}

func (region *memoryRegion) Size() int {
	return region.size
}

func memProtAndFlagsFromMode(mode int) (prot, flags int, err error) {
	switch mode {
	case MEM_READ_ONLY:
		prot = unix.PROT_READ
		flags = unix.MAP_SHARED
	case MEM_READ_PRIVATE:
		prot = unix.PROT_READ
		flags = unix.MAP_PRIVATE
	case MEM_READWRITE:
		prot = unix.PROT_READ | unix.PROT_WRITE
		flags = unix.MAP_SHARED
	case MEM_COPY_ON_WRITE:
		prot = unix.PROT_READ | unix.PROT_WRITE
		flags = unix.MAP_PRIVATE
	default:
		err = fmt.Errorf("invalid memory region flags %d", mode)
	}
	return
}

// syscalls
func msync(data []byte, flags int) error {
	dataPointer := unsafe.Pointer(&data[0])
	_, _, err := unix.Syscall(unix.SYS_MSYNC, uintptr(dataPointer), uintptr(len(data)), uintptr(flags))
	allocator.Use(dataPointer)
	if err != syscall.Errno(0) {
		return err
	}
	return nil
}
