package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Utils struct {
}

func (u *Utils) runCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)

	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		commandWithArgs := []string{command}
		commandWithArgs = append(commandWithArgs, args...)

		return errors.New(fmt.Sprintf("error running command: %v, err: %v, output: %v", strings.Join(commandWithArgs, " "), err, string(stdoutStderr)))
	}

	return nil
}
