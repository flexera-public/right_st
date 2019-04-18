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
	shebang              = regexp.MustCompile(`(?m)^#!.*$`)
	separator            = regexp.MustCompile(`[-_]`)
	rubyVariable         = regexp.MustCompile(`ENV\[["']([A-Z][A-Z0-9_]*)["']\]`)
	rubyAttachment       = regexp.MustCompile(`#\{ENV\[["']RS_ATTACH_DIR["']\]\}[/\\]+([^\t\n\f\r "]+)`)
	perlVariable         = regexp.MustCompile(`\$ENV\{["']?([A-Z][A-Z0-9_]*)["']?\}`)
	perlAttachment       = regexp.MustCompile(`\$ENV\{["']?RS_ATTACH_DIR["']?\}[/\\]+([^\t\n\f\r "]+)`)
	powershellVariable   = regexp.MustCompile(`\$\{?(?i:ENV):([A-Z][A-Z0-9_]*)\}?`)
	powershellAttachment = regexp.MustCompile(`\$\{?(?i:ENV):RS_ATTACH_DIR\}?[/\\]+([^\t\n\f\r "]+)`)
	shellVariable        = regexp.MustCompile(`\$\{?([A-Z][A-Z0-9_]*)(?::=([^}]*))?\}?`)
	shellAttachment      = regexp.MustCompile(`\$\{?RS_ATTACH_DIR(?::=[^}]*)?\}?[/\\]+([^\t\n\f\r "]+)`)
	ignoreVariables      = regexp.MustCompile(`^(?:ATTACH_DIR|BASH_REMATCH|SHELL|TERM|USER|PATH|MAIL|PWD|HOME|RS_.*|INSTANCE_ID|PRIVATE_ID|DATACENTER|EC2_.*)$`)
)

const (
	PreMetadata = iota
	InMetadata
	PostMetadata
)

func ScaffoldRightScript(path string, backup bool, stdout io.Writer, force bool) error {
	scriptBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}

	metadata, err := ParseRightScriptMetadata(bytes.NewReader(scriptBytes))
	if err != nil {
		return err
	}
	if metadata != nil {
		if !force {
			fmt.Fprintf(stdout, "%s: Script unchanged, already contains metadata. Use --force to force redetection.\n", path)
			return nil
		}
	} else {
		metadata = &RightScriptMetadata{
			Name:        strings.Title(separator.ReplaceAllLiteralString(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), " ")),
			Description: "(put your description here, it can be multiple lines using YAML syntax)",
			Inputs:      InputMap{},
		}
	}

	scaffoldedScriptBytes, err := scaffoldBuffer(scriptBytes, *metadata, path, true)
	if err != nil {
		return err
	}

	if backup {
		err := ioutil.WriteFile(path+".bak", scriptBytes, stat.Mode())
		if err != nil {
			return err
		}
	}
	err = ioutil.WriteFile(path, scaffoldedScriptBytes, stat.Mode())
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%s: Added metadata\n", path)
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
func scaffoldBuffer(source []byte, defaults RightScriptMetadata, filename string, detectInputs bool) ([]byte, error) {
	// We simply start with the defaults passed in as our base set of metadata.
	// Merging of defaults with exisiting metadata items happens before this function as strategies will be different
	// based on the source.
	metadata := &defaults

	variable := shellVariable
	attachment := shellAttachment
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".rb":
		variable = rubyVariable
		attachment = rubyAttachment
	case ".pl":
		variable = perlVariable
		attachment = perlAttachment
	case ".ps1":
		variable = powershellVariable
		attachment = powershellAttachment
	}

	// Pass 1: We remove any existing metadata comments and record the line at which we
	// removed them, so that we may re-insert them later.
	inMetadataState := PreMetadata
	metadataStartLine := 0
	scanner := bufio.NewScanner(bytes.NewReader(source))
	var buffer bytes.Buffer
	for lineCount := 0; scanner.Scan(); lineCount += 1 {
		line := scanner.Text()

		if inMetadataState == PreMetadata && metadataStart.MatchString(line) {
			metadataStartLine = lineCount
			inMetadataState = InMetadata
		} else if inMetadataState == InMetadata && metadataEnd.MatchString(line) {
			inMetadataState = PostMetadata
		} else {
			if inMetadataState != InMetadata {
				buffer.WriteString(line + "\n")
			}
		}
	}
	if inMetadataState == PostMetadata {
		source = buffer.Bytes() // we encountered metadata
	} else {
		metadataStartLine = 0 // we didn't encounter metadata
	}
	scanner = bufio.NewScanner(bytes.NewReader(source))

	// Pass 2: We autodetect all inputs. If we didn't autodetect metadata before we calculate the insertion point
	// as being after the shebang
	seenNames := make(map[string]bool)

	for lineCount := 0; scanner.Scan(); lineCount += 1 {
		line := scanner.Text()
		if lineCount == 0 {
			if shebang.MatchString(line) {
				switch {
				case strings.Contains(line, "ruby"):
					variable = rubyVariable
					attachment = rubyAttachment
				case strings.Contains(line, "perl"):
					variable = perlVariable
					attachment = perlAttachment
				}
				if metadataStartLine == 0 {
					metadataStartLine = 1
				}
				continue
			}
		}

		// We don't want to redetect for RightScripts -- users may have left out inputs on purpose to ignore them.
		if !detectInputs {
			continue
		}

		for _, submatches := range variable.FindAllStringSubmatch(line, -1) {
			name := submatches[1]
			seenNames[name] = true

			if ignoreVariables.MatchString(name) {
				continue
			}
			foundInput := false
			for _, i := range metadata.Inputs {
				if i.Name == name {
					foundInput = true
				}
			}

			if !foundInput {
				newInput := InputMetadata{
					Name:        name,
					Category:    "(put your input category here)",
					Description: "(put your input description here, it can be multiple lines using YAML syntax)",
				}
				metadata.Inputs = append(metadata.Inputs, newInput)
			}

			var inputItem *InputMetadata
			for idx, input := range metadata.Inputs {
				if input.Name == name {
					inputItem = &metadata.Inputs[idx]
				}
			}

			if len(submatches) > 2 && submatches[2] != "" && inputItem.Default == nil {
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

				inputItem.InputType = inputType
				inputItem.Default = &inputValue
			}
		}

	ATTACHMENTS:
		for _, submatches := range attachment.FindAllStringSubmatch(line, -1) {
			attachmentName := submatches[1]
			for _, attachment := range metadata.Attachments {
				if attachment == attachmentName {
					continue ATTACHMENTS
				}
			}
			metadata.Attachments = append(metadata.Attachments, attachmentName)
		}
	}
	if detectInputs {
		// Now remove any inputs that might have been deleted
		inputs := InputMap{}
		for _, in := range metadata.Inputs {
			if seenNames[in.Name] {
				inputs = append(inputs, in)
			}
		}
		metadata.Inputs = inputs
	}

	if err := scanner.Err(); err != nil {
		return []byte{}, err
	}

	// Pass 3: Create a new buffer with the metadata inserted at the right point.
	scanner = bufio.NewScanner(bytes.NewReader(source))
	script := bytes.Buffer{}
	for lineCount := 0; scanner.Scan(); lineCount += 1 {
		line := scanner.Text()
		if lineCount == metadataStartLine {
			metadata.WriteTo(&script)
		}
		script.WriteString(line + "\n")
	}
	if len(script.Bytes()) == 0 {
		metadata.WriteTo(&script)
	}

	return script.Bytes(), nil
}
