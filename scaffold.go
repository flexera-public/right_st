package main

import (
	"os"
)

func AddRightScriptMetadata(path string) error {
	script, err := os.Open(path)
	if err != nil {
		return err
	}
	defer script.Close()

	// check if metadata already exists

	// Load script

	// Add metadata to buffer version

	// write to file

	return nil
}
