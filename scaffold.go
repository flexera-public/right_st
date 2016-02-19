package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	shebang            = regexp.MustCompile(`^#!.*$`)
	separator          = regexp.MustCompile(`[-_]`)
	rubyVariable       = regexp.MustCompile(`ENV\[["']([A-Z][A-Z0-9_]*)["']\]`)
	perlVariable       = regexp.MustCompile(`\$ENV\{["']?([A-Z][A-Z0-9_]*)["']?\}`)
	powershellVariable = regexp.MustCompile(`\$\{?(?i:ENV):([A-Z][A-Z0-9_]*)\}?`)
	shellVariable      = regexp.MustCompile(`\$\{?([A-Z][A-Z0-9_]*)(?::=([^}]*))?\}?`)
	ignoreVariables    = regexp.MustCompile(`^(?:ATTACH_DIR|SHELL|TERM|USER|PATH|MAIL|PWD|HOME|RS_.*|INSTANCE_ID|PRIVATE_ID|DATACENTER|EC2_.*)$`)
)

func ScaffoldRightScript(path string, backup bool, stdout io.Writer) error {
	script, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer script.Close()
	return ScaffoldRightScriptFile(script, backup, stdout)
}

func ScaffoldRightScriptFile(script *os.File, backup bool, stdout io.Writer) error {
	path := script.Name()
	if backup {
		backupScript, err := os.Create(path + ".bak")
		if err != nil {
			return err
		}
		defer backupScript.Close()

		_, err = io.Copy(backupScript, script)
		if err != nil {
			return err
		}
		err = backupScript.Close()
		if err != nil {
			return err
		}
		_, err = script.Seek(0, os.SEEK_SET)
		if err != nil {
			return err
		}
	}

	metadata, err := ParseRightScriptMetadata(script)
	if err != nil {
		return err
	}
	if metadata != nil {
		fmt.Fprintf(stdout, "%s: Already contains metadata\n", path)
		return nil
	}

	inputs := make(InputMap)
	metadata = &RightScriptMetadata{
		Name:        strings.Title(separator.ReplaceAllLiteralString(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), " ")),
		Description: "(put your description here, it can be multiple lines using YAML syntax)",
		Inputs:      &inputs,
	}

	variable := shellVariable
	switch strings.ToLower(filepath.Ext(path)) {
	case ".rb":
		variable = rubyVariable
	case ".pl":
		variable = perlVariable
	case ".ps1":
		variable = powershellVariable
	}

	scanner := bufio.NewScanner(script)
	shebangEnd := -1
	var buffer bytes.Buffer
	for scanner.Scan() {
		line := scanner.Text()
		if shebangEnd < 0 {
			if shebang.MatchString(line) {
				shebangEnd = len(line)
				switch {
				case strings.Contains(line, "ruby"):
					variable = rubyVariable
				case strings.Contains(line, "perl"):
					variable = perlVariable
				}
				continue
			} else {
				shebangEnd = 0
			}
		}

		for _, submatches := range variable.FindAllStringSubmatch(line, -1) {
			name := submatches[1]
			if ignoreVariables.MatchString(name) {
				continue
			}
			inputs = *metadata.Inputs
			if _, ok := inputs[name]; !ok {
				inputs[name] = &InputMetadata{
					Category:    "(put your input category here)",
					Description: "(put your input description here, it can be multiple lines using YAML syntax)",
				}
			}
			if len(submatches) > 2 && submatches[2] != "" && inputs[name].Default == nil {
				values := strings.Split(submatches[2], ",")
				var (
					inputType  InputType
					inputValue InputValue
				)
				if len(values) == 1 {
					inputType = Single
					inputValue = InputValue{Type: "text", Value: values[0]}
				} else {
					array := make([]string, len(values))
					for index, value := range values {
						array[index] = fmt.Sprintf("%q", InputValue{Type: "text", Value: value})
					}
					inputType = Array
					inputValue = InputValue{Type: "array", Value: "[" + strings.Join(array, ",") + "]"}
				}
				inputs[name].InputType = inputType
				inputs[name].Default = &inputValue
			}
		}

		buffer.WriteString(line + "\n")
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if shebangEnd < 0 {
		shebangEnd = 0
	}

	_, err = script.Seek(int64(shebangEnd), os.SEEK_SET)
	if err != nil {
		return err
	}
	err = script.Truncate(int64(shebangEnd))
	if err != nil {
		return err
	}
	if shebangEnd != 0 {
		_, err = script.WriteString("\n")
		if err != nil {
			return err
		}
	}
	_, err = metadata.WriteTo(script)
	if err != nil {
		return err
	}
	_, err = buffer.WriteTo(script)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s: Added metadata\n", path)
	return nil
}
