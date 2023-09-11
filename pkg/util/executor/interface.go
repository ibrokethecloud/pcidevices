package executor

import "os/exec"

type Executor interface {
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

func (l *LocalExecutor) Run(cmd string, args []string) ([]byte, error) {
	localCommand := exec.Command(cmd, args...)
	localCommand.Env = append(localCommand.Env, l.envVars...)
	return localCommand.Output()
}
