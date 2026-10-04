package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type ContainerInfo struct {
	Name      string
	PID       int
	MemoryMB  int
	CreatedAt time.Time
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
