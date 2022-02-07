// Copyright 2015 Aleksandr Demakin. All rights reserved.

package mmf

import (
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"github.com/avdva/go-ipc/internal/allocator"
	sys "github.com/avdva/go-ipc/internal/sys/windows"

	"golang.org/x/sys/windows"
)

func init() {
	g, p := sys.GetAllocGranularity(), os.Getpagesize()
	if g >= p {
		mmapOffsetMultiple = int64(g)
	} else {
		mmapOffsetMultiple = int64(p)
	}
}

type memoryRegion struct {
	data       []byte
	size       int
	pageOffset int64
}

type native interface {
	IsNative() bool
}

func newMemoryRegion(obj Mappable, mode int, offset int64, size int) (*memoryRegion, error) {
	prot, flags, err := sysProtAndFlagsFromFlag(mode)
	if err != nil {
		return nil, fmt.Errorf("checking memory region flags: %w", err)
	}
	if size, err = checkMmapSize(obj, size); err != nil {
		return nil, fmt.Errorf("size check failed: %w", err)
	}
	maxSizeHigh := uint32((offset + int64(size)) >> 32)
	maxSizeLow := uint32((offset + int64(size)) & 0xFFFFFFFF)

	handle := windows.Handle(obj.Fd())

	// check for a named region for windows native shared memory via a pagefile.
	// in this case there is no need to create a mapping file.
	if nativeObj, ok := obj.(native); !ok || !nativeObj.IsNative() {
		handle, err = sys.CreateFileMapping(
			handle,
			nil,
			prot,
			maxSizeHigh,
			maxSizeLow,
			"")
		if err != nil {
			return nil, fmt.Errorf("creating mapping file: %w", err)
		}
		defer windows.CloseHandle(handle)
	}

	pageOffset := calcMmapOffsetFixup(offset)
	offset -= pageOffset
	lowOffset := uint32(offset & 0xFFFFFFFF)
	highOffset := uint32(offset >> 32)

	addr, err := windows.MapViewOfFile(handle, flags, highOffset, lowOffset, uintptr(int64(size)+pageOffset))
	if err != nil {
		return nil, fmt.Errorf("mmaping file view: %w", os.NewSyscallError("MapViewOfFile", err))
	}

	totalSize := size + int(pageOffset)
	return &memoryRegion{
		data:       allocator.ByteSliceFromUnsafePointer(unsafe.Pointer(addr), totalSize, totalSize),
		size:       size,
		pageOffset: pageOffset,
	}, nil
}

func (region *memoryRegion) Close() error {
	runtime.SetFinalizer(region, nil)
	err := windows.UnmapViewOfFile(uintptr(allocator.ByteSliceData(region.data)))
	if err != nil {
		return fmt.Errorf("UnmapViewOfFile: %w", err)
	}
	region.data = nil
	return nil
}

func (region *memoryRegion) Data() []byte {
	return region.data[region.pageOffset:]
}

func (region *memoryRegion) Size() int {
	return region.size
}

func (region *memoryRegion) Flush(async bool) error {
	err := windows.FlushViewOfFile(uintptr(allocator.ByteSliceData(region.data)), uintptr(len(region.data)))
	if err != nil {
		return fmt.Errorf("FlushViewOfFile: %w", err)
	}
	return nil
}

func sysProtAndFlagsFromFlag(mode int) (prot, flags uint32, err error) {
	switch mode {
	case MEM_READ_ONLY:
		fallthrough
	case MEM_READ_PRIVATE:
		prot = windows.PAGE_READONLY
		flags = windows.FILE_MAP_READ
	case MEM_READWRITE:
		prot = windows.PAGE_READWRITE
		flags = windows.FILE_MAP_WRITE
	case MEM_COPY_ON_WRITE:
		prot = windows.PAGE_WRITECOPY
		flags = windows.FILE_MAP_COPY
	default:
		err = fmt.Errorf("invalid mem region flags %d", mode)
	}
	return
}
