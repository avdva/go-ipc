// Copyright 2016 Aleksandr Demakin. All rights reserved.

package helper

import (
	"fmt"
	"os"

	"github.com/avdva/go-ipc/mmf"
	"github.com/avdva/go-ipc/shm"
)

// CreateWritableRegion is a helper, which:
//	- creates a shared memory object with given parameters.
//	- creates a mapping for the entire region with mmf.MEM_READWRITE flag.
//	- closes memory object and returns memory region and a flag whether the object was created.
func CreateWritableRegion(name string, flag int, perm os.FileMode, size int) (*mmf.MemoryRegion, bool, error) {
	obj, created, resultErr := shm.NewMemoryObjectSize(name, flag, perm, int64(size))
	if resultErr != nil {
		return nil, false, fmt.Errorf("creating shm object: %w", resultErr)
	}
	var region *mmf.MemoryRegion
	defer func() {
		obj.Close()
		if resultErr == nil {
			return
		}
		if region != nil {
			region.Close()
		}
		if created {
			obj.Destroy()
		}
	}()
	if region, resultErr = mmf.NewMemoryRegion(obj, mmf.MEM_READWRITE, 0, size); resultErr != nil {
		return nil, false, fmt.Errorf("creating shm region: %w", resultErr)
	}
	return region, created, nil
}
