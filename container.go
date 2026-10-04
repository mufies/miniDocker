package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

func parent() {
	mb, rest := parseMemoryFlag(os.Args[2:])
	name, rest2 := parseNameFlag(rest)
	cmd := exec.Command("/proc/self/exe", append([]string{"child", name}, rest2...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Start()
	if err != nil {
		fmt.Println("Lỗi start:", err)
		return
	}

	err = setupNetwork(cmd.Process.Pid, name)
	if err != nil {
		fmt.Println("Loi network", err)
	}

	err = setupCgroup(cmd.Process.Pid, mb)
	if err != nil {
		fmt.Println("Lỗi cgroup:", err)
	}
	info := ContainerInfo{
		Name:      name,
		PID:       cmd.Process.Pid,
		MemoryMB:  mb,
		CreatedAt: time.Now(),
	}
	err = saveContainerInfo(info)
	if err != nil {
		fmt.Println("Lỗi lưu state:", err)
	}
	defer os.RemoveAll("/sys/fs/cgroup/minidocker")
	err = cmd.Wait()
	if err != nil {
		fmt.Println(err)
	}
}

func child() {
	containerName := os.Args[2]

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

	err = setupContainerNetwork(containerName)
	if err != nil {
		fmt.Println("Lỗi network container:", err)
	}

	binary, err := exec.LookPath(os.Args[3])
	if err != nil {
		fmt.Println(err)
		return
	}

	err = syscall.Exec(binary, os.Args[3:], os.Environ())
	if err != nil {
		fmt.Println(err)
	}
}

func parseMemoryFlag(args []string) (int, []string) {
	memoryMB := 50
	remaining := []string{}

	for i := 0; i < len(args); i++ {
		if args[i] == "--memory" {
			value, err := strconv.Atoi(args[i+1])
			if err != nil {
				fmt.Println("Lỗi parse --memory:", err)
			} else {
				memoryMB = value
			}
			i++
			continue
		}
		remaining = append(remaining, args[i])
	}

	return memoryMB, remaining
}

func parseNameFlag(args []string) (string, []string) {
	name := fmt.Sprintf("container-%d", time.Now().Unix())
	remaining := []string{}

	for i := 0; i < len(args); i++ {
		if args[i] == "--name" {
			name = args[i+1]
			i++
			continue
		}
		remaining = append(remaining, args[i])
	}
	return name, remaining
}
