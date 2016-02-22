package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
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
	scriptBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}

	inputs := make(InputMap)
	metadata := RightScriptMetadata{
		Name:        strings.Title(separator.ReplaceAllLiteralString(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), " ")),
		Description: "(put your description here, it can be multiple lines using YAML syntax)",
		Inputs:      &inputs,
	}

	scaffoldedScript, err := scaffoldBuffer(scriptBytes, metadata, path)
	if err != nil {
		return err
	}
	scaffoldedScriptBytes, _ := ioutil.ReadAll(scaffoldedScript)
	if bytes.Compare(scaffoldedScriptBytes, scriptBytes) == 0 {
		fmt.Fprintf(stdout, "%s: Script unchanged, already contains metadata\n", path)
	} else {
		if backup {
			err := ioutil.WriteFile(path+".bak", scriptBytes, stat.Mode())
			if err != nil {
				return err
			}
		}
		err := ioutil.WriteFile(path, scaffoldedScriptBytes, stat.Mode())
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s: Added metadata\n", path)
	}
	return nil
}

// Params:
//   source - Source buffer. Will not be modified
//   defaults - Default values. Parsed values will be merged in.
//   filename - Used to help determine the script type.
// Return values:
//   *bytes.Buffer - New buffer with added metadata. Currently metadata will only be added if there is none. We don't
//                   currently bother with any fancy merging or updating if new inputs/attachments get added in the API or disk
//   err error - error value
func scaffoldBuffer(source []byte, defaults RightScriptMetadata, filename string) (*bytes.Buffer, error) {
	metadata, err := ParseRightScriptMetadata(bytes.NewReader(source))
	if err != nil {
		return nil, err
	}
	if metadata != nil {
		return bytes.NewBuffer(source), nil
	}
	// TBD merge defaults into metadata or vice versa. For now, just start with the defaults
	metadata = &defaults

	variable := shellVariable
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".rb":
		variable = rubyVariable
	case ".pl":
		variable = perlVariable
	case ".ps1":
		variable = powershellVariable
	}

	scanner := bufio.NewScanner(bytes.NewReader(source))
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
			inputs := *metadata.Inputs
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
		return nil, err
	}

	if shebangEnd < 0 {
		shebangEnd = 0
	}
	scriptBytes := make([]byte, len(source))
	copy(scriptBytes, source)
	script := bytes.NewBuffer(scriptBytes)
	script.Truncate(shebangEnd)

	if shebangEnd != 0 {
		_, err = script.WriteString("\n")
		if err != nil {
			return nil, err
		}
	}
	_, err = metadata.WriteTo(script)
	if err != nil {
		return nil, err
	}
	_, err = buffer.WriteTo(script)
	if err != nil {
		return nil, err
	}

	return script, nil
}
