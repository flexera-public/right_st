// +build !windows

package main

import (
	"os/user"
	"path/filepath"
)

func defaultConfigFile() string {
	currentUser, err := user.Current()
	if err != nil {
		panic(err)
	}

	return filepath.Join(currentUser.HomeDir, ".right_st.yml")
}
