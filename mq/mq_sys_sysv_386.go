// Copyright 2016 Aleksandr Demakin. All rights reserved.

// +build darwin dragonfly freebsd linux netbsd openbsd solaris
// +build 386

package mq

import (
	"os"
	"syscall"
	"unsafe"

	"bitbucket.org/avd/go-ipc/internal/allocator"

	"golang.org/x/sys/unix"
)

const (
	cMSGSND = 11
	cMSGRCV = 12
	cMSGGET = 13
	cMSGCTL = 14
)

func msgget(k key, flags int) (int, error) {
	id, _, err := syscall.Syscall6(unix.SYS_IPC, uintptr(cMSGGET), uintptr(k), uintptr(flags), 0, 0, 0)
	if err != syscall.Errno(0) {
		return 0, os.NewSyscallError("MSGGET", err)
	}
	return int(id), nil
}

func msgsnd(id int, typ int, data []byte, flags int) error {
	messageLen := typeDataSize + len(data)
	message := make([]byte, messageLen)
	rawData := allocator.ByteSliceData(message)
	*(*int)(unsafe.Pointer(rawData)) = typ
	copy(message[typeDataSize:], data)
	_, _, err := syscall.Syscall6(unix.SYS_IPC,
		uintptr(cMSGSND),
		uintptr(id),
		uintptr(len(data)),
		uintptr(flags),
		uintptr(rawData),
		0)
	allocator.Use(rawData)
	if err != syscall.Errno(0) {
		return os.NewSyscallError("MSGSND", err)
	}
	return nil
}

func msgrcv(id int, data []byte, typ int, flags int) error {
	messageLen := typeDataSize + len(data)
	message := make([]byte, messageLen)
	rawData := allocator.ByteSliceData(message)
	_, _, err := syscall.Syscall6(unix.SYS_IPC,
		uintptr(cMSGRCV|(1<<16)),
		uintptr(id),
		uintptr(len(data)),
		uintptr(flags),
		uintptr(rawData),
		uintptr(typ))
	allocator.Use(rawData)
	copy(data, message[typeDataSize:])
	if err != syscall.Errno(0) {
		return os.NewSyscallError("MSGRCV", err)
	}
	return nil
}

func msgctl(id int, cmd int, buf *msqidDs) error {
	_, _, err := syscall.Syscall(unix.SYS_IPC, uintptr(cMSGCTL), uintptr(id), uintptr(cmd))
	if err != syscall.Errno(0) {
		return os.NewSyscallError("MSGCTL", err)
	}
	return nil
}
