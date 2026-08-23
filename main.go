// 根入口把 `go run .` 转发到实际服务命令。
package main

import (
	"log"
	"os"
	"os/exec"
)

func main() {
	cmd := exec.Command("go", "run", "./cmd/saltern-watch")
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
