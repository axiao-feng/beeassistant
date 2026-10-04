//go:build windows

package command

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	cmd.Cancel = func() error {
		return exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprint(cmd.Process.Pid)).Run()
	}
}

// startBackgroundProcess 直接启动 cmd.exe，stdout/stderr 通过文件句柄重定向。
// 不把命令、工作目录和输出路径拼接进 PowerShell 脚本，避免特殊字符触发额外解析。
func startBackgroundProcess(command, workDir string) (*backgroundProcessResult, error) {
	stdoutFile, err := os.CreateTemp(workDir, "bg_stdout_*.txt")
	if err != nil {
		return nil, err
	}
	stdoutPath := stdoutFile.Name()

	stderrFile, err := os.CreateTemp(workDir, "bg_stderr_*.txt")
	if err != nil {
		stdoutFile.Close()
		os.Remove(stdoutPath)
		return nil, err
	}
	stderrPath := stderrFile.Name()

	shell := os.Getenv("COMSPEC")
	if shell == "" {
		shell = "cmd.exe"
	}
	cmd := exec.Command(shell, "/D", "/S", "/C", command)
	cmd.Dir = workDir
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow,
	}
	if err := cmd.Start(); err != nil {
		stdoutFile.Close()
		stderrFile.Close()
		os.Remove(stdoutPath)
		os.Remove(stderrPath)
		return nil, err
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	stdoutFile.Close()
	stderrFile.Close()
	return &backgroundProcessResult{PID: pid, StdoutFile: stdoutPath, StderrFile: stderrPath}, nil
}
