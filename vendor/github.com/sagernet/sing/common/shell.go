package common

import (
	"os"
	"os/exec"
)

type Shell struct {
	*exec.Cmd
}

func Exec(name string, args ...string) *Shell {
	command := exec.Command(name, args...)
	command.Env = os.Environ()
	return &Shell{command}
}

func (s *Shell) SetDir(path string) *Shell {
	s.Dir = path
	return s
}

func (s *Shell) Attach() *Shell {
	s.Stdin = os.Stdin
	s.Stdout = os.Stderr
	s.Stderr = os.Stderr
	return s
}

func (s *Shell) SetEnv(env []string) *Shell {
	s.Env = append(os.Environ(), env...)
	return s
}

func (s *Shell) Read() (string, error) {
	output, err := s.CombinedOutput()
	return string(output), err
}
