package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("cần truyền lệnh: ./minidocker <run|child|ps|stop> [args...]")
		return
	}

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
	default:
		fmt.Println("Lệnh không hợp lệ:", os.Args[1])
	}
}
