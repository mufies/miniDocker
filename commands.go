package main

import (
	"fmt"
	"os"

	"github.com/vishvananda/netlink"
)

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
