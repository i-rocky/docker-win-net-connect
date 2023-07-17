package main

import (
	"errors"
	"os/exec"
)

type Utils struct {
}

func (u *Utils) runCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	err := cmd.Run()

	if err != nil {
		return errors.New("error running command " + err.Error())
	}

	return nil
}
