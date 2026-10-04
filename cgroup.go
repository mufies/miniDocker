package main

import (
	"fmt"
	"os"
)

func setupCgroup(pid int, memoryLimitMB int) error {
	cgroupPath := "/sys/fs/cgroup/minidocker"

	err := os.MkdirAll(cgroupPath, 0o755)
	if err != nil {
		return err
	}
	memoryLimit := fmt.Sprintf("%d", memoryLimitMB*1024*1024)
	err = os.WriteFile(cgroupPath+"/memory.max", []byte(memoryLimit), 0o644)
	if err != nil {
		return err
	}

	pidStr := fmt.Sprintf("%d", pid)
	err = os.WriteFile(cgroupPath+"/cgroup.procs", []byte(pidStr), 0o644)
	if err != nil {
		return err
	}

	return nil
}
