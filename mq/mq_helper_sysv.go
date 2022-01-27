// Copyright 2016 Aleksandr Demakin. All rights reserved.

//go:build (darwin || freebsd || linux) && !linux_mq && !fast_mq
// +build darwin freebsd linux
// +build !linux_mq
// +build !fast_mq

package mq

import "os"

func createMQ(name string, flag int, perm os.FileMode) (Messenger, error) {
	mq, err := CreateSystemVMessageQueue(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return mq, nil
}

func openMQ(name string, flag int) (Messenger, error) {
	mq, err := OpenSystemVMessageQueue(name, flag)
	if err != nil {
		return nil, err
	}
	return mq, nil
}

func destroyMq(name string) error {
	return DestroySystemVMessageQueue(name)
}
