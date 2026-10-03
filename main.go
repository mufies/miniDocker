package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	switch os.Args[1] {
	case "run":
		parent()

	case "child":
		child()
	}
}

func parent() {
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Start()
	if err != nil {
		fmt.Println("Lỗi start:", err)
		return
	}

	err = setupNetwork(cmd.Process.Pid)
	if err != nil {
		fmt.Println("Loi network", err)
	}
	err = setupCgroup(cmd.Process.Pid, 50)
	if err != nil {
		fmt.Println("Lỗi cgroup:", err)
	}
	defer os.RemoveAll("/sys/fs/cgroup/minidocker")
	err = cmd.Wait()
	if err != nil {
		fmt.Println(err)
	}
}

func child() {
	syscall.Sethostname([]byte("minicontainer"))
	err := syscall.Chroot("/home/mufi/mydocker-rootfs")
	if err != nil {
		fmt.Println("Lỗi chroot:", err)
		return
	}

	err = os.Chdir("/")
	if err != nil {
		fmt.Println("Lỗi chdir:", err)
		return
	}
	err = syscall.Mount("proc", "/proc", "proc", 0, "")
	if err != nil {
		fmt.Println("Lỗi mount /proc:", err)
		return
	}
	binary, err := exec.LookPath(os.Args[2])
	if err != nil {
		fmt.Println(err)
		return
	}
	err = setupContainerNetwork()
	if err != nil {
		fmt.Println("Lỗi network container:", err)
	}
	err = syscall.Exec(binary, os.Args[2:], os.Environ())
	if err != nil {
		fmt.Println(err)
	}
}

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
