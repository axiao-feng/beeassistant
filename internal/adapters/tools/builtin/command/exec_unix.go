//go:build !windows

package command

import (
	"os"
	"os/exec"
	"syscall"
)

func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// 取消时 kill 整个进程组，避免子进程残留
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// startBackgroundProcess 直接启动受控 shell，stdout/stderr 写入临时文件。
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

	shell, shellArgs := buildShellCommand(command)
	cmd := exec.Command(shell, shellArgs...)
	cmd.Dir = workDir
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile
	setupProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		stdoutFile.Close()
		stderrFile.Close()
		os.Remove(stdoutPath)
		os.Remove(stderrPath)
		return nil, err
	}
	pid := cmd.Process.Pid
	return &backgroundProcessResult{
		PID:        pid,
		StdoutFile: stdoutPath,
		StderrFile: stderrPath,
		Wait:       cmd.Wait,
		Terminate: func() error {
			return syscall.Kill(-pid, syscall.SIGKILL)
		},
	}, nil
}
