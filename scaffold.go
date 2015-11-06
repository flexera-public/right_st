package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	shebang = regexp.MustCompile(`^(#!\s?.*)$`)
)

func AddRightScriptMetadata(path string) error {
	readScript, err := os.Open(path)
	if err != nil {
		return err
	}
	defer readScript.Close()

	// check if metadata section set by delimiters already exists
	// at the same time load and scan script in line by line to obtain inputs
	scanner := bufio.NewScanner(readScript)
	inMetadata := false
	metadataExists := false
	var buffer bytes.Buffer
	var metadatabuffer bytes.Buffer
	// TODO: quick hack for now, but use yaml tools to build metadata
	initialMetadataString := fmt.Sprintf("# ---\n# RightScript Name: %s\n# Description:\n# Inputs:\n# ...\n\n", strings.TrimRight(filepath.Base(path), filepath.Ext(path)))
	for scanner.Scan() {
		line := scanner.Text()

		// If first line, check for shebang
		if metadatabuffer.Len() == 0 {
			submatches := shebang.FindStringSubmatch(line)
			if submatches != nil {
				metadatabuffer.WriteString(submatches[1] + "\n")
			} else {
				buffer.WriteString(line + "\n")
			}
			metadatabuffer.WriteString(initialMetadataString)
		} else {
			buffer.WriteString(line + "\n")
		}

		// TODO: Check for inputs in 'line' and append to metadatabuffer

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

	if metadataExists {
		fmt.Printf("%s - metadata already exists\n", path)
	} else {
		writeScript, err := os.Create(path)
		if err != nil {
			return err
		}
		fmt.Printf("%s - metadata added\n", path)

		defer writeScript.Close()
		metadatabuffer.WriteTo(writeScript)
		buffer.WriteTo(writeScript)

		// Add metadata to buffer version

		// write to file
	}

	return nil
}
