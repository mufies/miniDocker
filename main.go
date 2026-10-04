package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
)

type ContainerInfo struct {
	Name      string
	PID       int
	MemoryMB  int
	CreatedAt time.Time
}

func main() {
	switch os.Args[1] {
	case "run":
		parent()

	case "child":
		child()
	case "ps":
		psCommand()
	case "stop":
		if len(os.Args) < 3 {
			fmt.Println("cần truyền tên container: ./minidocker stop <name>")
			return
		}
		stopCommand(os.Args[2])
	}
}

func saveContainerInfo(info ContainerInfo) error {
	dir := "/var/run/minidocker"
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return err
	}

	data, err := json.Marshal(info)
	if err != nil {
		return err
	}

	path := dir + "/" + info.Name + ".json"
	return os.WriteFile(path, data, 0o644)
}

func loadAllContainers() ([]ContainerInfo, error) {
	dir := "/var/run/minidocker"

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var containers []ContainerInfo

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := dir + "/" + entry.Name()
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Println("Lỗi đọc file:", path, err)
			continue
		}

		var info ContainerInfo
		err = json.Unmarshal(data, &info)
		if err != nil {
			fmt.Println("Lỗi parse JSON:", path, err)
			continue
		}

		containers = append(containers, info)
	}

	return containers, nil
}

func psCommand() {
	containers, err := loadAllContainers()
	if err != nil {
		fmt.Println("Lỗi đọc container:", err)
		return
	}

	fmt.Printf("%-10s %-8s %-10s %-20s\n", "NAME", "PID", "MEMORY", "CREATED")
	for _, c := range containers {
		fmt.Printf("%-10s %-8d %-10d %-20s\n", c.Name, c.PID, c.MemoryMB, c.CreatedAt.Format("2006-01-02 15:04:05"))
	}
}

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

func stopCommand(name string) {
	containers, err := loadAllContainers()
	if err != nil {
		fmt.Println("Lỗi đọc container:", err)
		return
	}

	var target *ContainerInfo
	for i := range containers {
		if containers[i].Name == name {
			target = &containers[i]
			break
		}
	}

	if target == nil {
		fmt.Println("Không tìm thấy container:", name)
		return
	}

	// 1. Kill process
	process, err := os.FindProcess(target.PID)
	if err == nil {
		err = process.Kill()
		if err != nil {
			fmt.Println("Lỗi kill process:", err)
		}
	}

	// 2. Xóa veth ở host
	vethHostName := "veth-" + name
	link, err := netlink.LinkByName(vethHostName)
	if err == nil {
		netlink.LinkDel(link)
	}

	// 3. Xóa cgroup
	os.RemoveAll("/sys/fs/cgroup/minidocker")

	// 4. Xóa file JSON state
	os.Remove("/var/run/minidocker/" + name + ".json")

	fmt.Println("Đã dừng container:", name)
}
