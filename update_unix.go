//go:build !windows
// +build !windows

package main

import (
	"syscall"

	"github.com/kardianos/osext"
	"golang.org/x/sys/unix"
)

// updateSudoCommand returns the right_st command name with any sudo prefix if necessary. The command name is returned
// even if an error occurred.
func updateSudoCommand() (string, error) {
	// get the full path to the right_st executable so its access can be checked
	exe, err := osext.Executable()
	if err != nil {
		return app.Name, err
	}

	// check if the right_st executable can be written by the current user
	err = unix.Access(exe, unix.W_OK)
	if err == syscall.EACCES {
		// the right_st executable cannot be written by the current user so prefix the command name with sudo
		return "sudo " + app.Name, nil
	}

	return app.Name, err
}
