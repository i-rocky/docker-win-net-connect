package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Utils struct {
}

func (u *Utils) runCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()

	if err != nil {
		commandWithArgs := []string{command}
		commandWithArgs = append(commandWithArgs, args...)

		return errors.New(fmt.Sprintf("error running command: %v, err: %v ", strings.Join(commandWithArgs, " "), err))
	}

	return nil
}
