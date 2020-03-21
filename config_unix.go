// +build !windows

package main

import (
	"os/user"
	"path/filepath"
)

func DefaultConfigFile() string {
	currentUser, err := user.Current()
	if err != nil {
		panic(err)
	}

	return filepath.Join(currentUser.HomeDir, ".right_st.yml")
}

func DefaultCachePath() string {
	currentUser, err := user.Current()
	if err != nil {
		panic(err)
	}

	return filepath.Join(currentUser.HomeDir, ".right_st.cache")
}
