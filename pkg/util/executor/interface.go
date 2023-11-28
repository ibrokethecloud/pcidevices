package executor

import (
	"os/exec"
	"path/filepath"
)

type Executor interface {
	CheckReady() ([]byte, error)
	Run(cmd string, args []string) ([]byte, error)
}

type LocalExecutor struct {
	envVars []string
}

func NewLocalExecutor(envVars []string) *LocalExecutor {
	return &LocalExecutor{
		envVars: envVars,
	}
}

const (
	localExecutorAppend = "/host"
	sriovManageCommand  = "/usr/lib/nvidia/sriov-manage"
)

func (l *LocalExecutor) Run(cmd string, args []string) ([]byte, error) {
	// localExecutor is run inside pcidevices pod, so need to add `/host` to command
	cmd = filepath.Join(localExecutorAppend, cmd)
	localCommand := exec.Command(cmd, args...)
	localCommand.Env = append(localCommand.Env, l.envVars...)
	return localCommand.Output()
}

// CheckReady checks if /host/usr/lib/nvidia/sriov-manage exists
func (l *LocalExecutor) CheckReady() ([]byte, error) {
	return l.Run(filepath.Join(localExecutorAppend, "/usr/bin/file"), []string{filepath.Join(localExecutorAppend, sriovManageCommand)})
}
