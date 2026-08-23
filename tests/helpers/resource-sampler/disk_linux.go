//go:build linux

package main

import "syscall"

func freeBytes(path string) (uint64, error) {
	var value syscall.Statfs_t
	if err := syscall.Statfs(path, &value); err != nil {
		return 0, err
	}
	return value.Bavail * uint64(value.Bsize), nil
}
