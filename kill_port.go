package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Printf("Usage: go run kill_port.go <port>\n")
		fmt.Printf("Example: go run kill_port.go 10501\n")
		os.Exit(1)
	}

	port := os.Args[1]

	// 查找占用端口的进程
	pid, err := getPIDByPort(port)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if pid == 0 {
		fmt.Printf("No process found on port %s\n", port)
		os.Exit(0)
	}

	fmt.Printf("Found process on port %s: PID=%d\n", port, pid)

	// 尝试获取进程名称
	processName := getProcessName(pid)
	if processName != "" {
		fmt.Printf("Process name: %s\n", processName)
	}

	// 终止进程
	fmt.Printf("Killing process %d...\n", pid)
	if err := killProcess(pid); err != nil {
		fmt.Printf("Failed to kill process: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully killed process %d on port %s\n", pid, port)
}

// getPIDByPort 根据端口号获取进程 PID
func getPIDByPort(port string) (int, error) {
	// 使用 netstat 命令查找端口
	cmd := exec.Command("netstat", "-ano")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to run netstat: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// 查找监听该端口的行
		if strings.Contains(line, "LISTENING") && strings.Contains(line, ":"+port) {
			parts := strings.Fields(line)
			if len(parts) >= 5 {
				pidStr := parts[len(parts)-1]
				pid, err := strconv.Atoi(pidStr)
				if err == nil {
					return pid, nil
				}
			}
		}
	}

	return 0, nil
}

// getProcessName 根据 PID 获取进程名称
func getProcessName(pid int) string {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	parts := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(parts) >= 2 {
		name := strings.Trim(parts[0], "\"")
		return name
	}
	return ""
}

// killProcess 强制终止进程
func killProcess(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", string(output), err)
	}
	return nil
}
