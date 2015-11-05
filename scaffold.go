package main

import (
	"bufio"
	"fmt"
	"os"
)

func AddRightScriptMetadata(path string) error {
	script, err := os.Open(path)
	if err != nil {
		return err
	}
	defer script.Close()

	// check if metadata section set by delimiters already exists
	scanner := bufio.NewScanner(script)
	inMetadata := false
	metadataExists := false
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case inMetadata:
			submatches := metadataEnd.FindStringSubmatch(line)
			if submatches != nil {
				metadataExists = true
				break
			}
		case metadataStart.MatchString(line):
			inMetadata = true
		}
	}

	if metadataExists == true {
		fmt.Printf("%s - metadata already exists\n", path)
	}

	// Load script

	// Add metadata to buffer version

	// write to file

	return nil
}
