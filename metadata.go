package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-yaml/yaml"
)

const (
	Single InputType = iota
	Array
)

var (
	comment       = regexp.MustCompile(`^\s*(?:#|//|--)\s?(.*)$`)
	metadataStart = regexp.MustCompile(`^\s*(#|//|--)\s?(\s*-{3}\s*)$`)
	metadataEnd   = regexp.MustCompile(`^\s*(?:#|//|--)\s?(\s*\.{3}\s*)$`)
	yamlLineError = regexp.MustCompile(`^(yaml: )?line (\d+):`)
)

type RightScriptMetadata struct {
	Name        string                    `yaml:"RightScript Name"`
	Description string                    `yaml:"Description"`
	Inputs      map[string]*InputMetadata `yaml:"Inputs"`
	Attachments []string                  `yaml:"Attachments"`
	Comment     string                    `yaml:"-"`
}

type InputMetadata struct {
	Category       string        `yaml:"Category"`
	Description    string        `yaml:"Description"`
	InputType      InputType     `yaml:"Input Type"`
	Required       bool          `yaml:"Required"`
	Advanced       bool          `yaml:"Advanced"`
	Default        *InputValue   `yaml:"Default,omitempty"`
	PossibleValues []*InputValue `yaml:"Possible Values,omitempty"`
}

type InputType int

type InputValue struct {
	Type  string
	Value string
}

func ParseRightScriptMetadata(script io.ReadSeeker) (*RightScriptMetadata, error) {
	defer script.Seek(0, os.SEEK_SET)

	scanner := bufio.NewScanner(script)
	var buffer bytes.Buffer
	var lineNumber, offset uint
	inMetadata := false
	var metadata RightScriptMetadata

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		switch {
		case inMetadata:
			submatches := metadataEnd.FindStringSubmatch(line)
			if submatches != nil {
				buffer.WriteString(submatches[1] + "\n")
				inMetadata = false
				break
			}
			submatches = comment.FindStringSubmatch(line)
			if submatches != nil {
				buffer.WriteString(submatches[1] + "\n")
			}
		case metadataStart.MatchString(line):
			submatches := metadataStart.FindStringSubmatch(line)
			metadata.Comment = submatches[1]
			buffer.WriteString(submatches[2] + "\n")
			inMetadata = true
			offset = lineNumber
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if inMetadata {
		return nil, fmt.Errorf("Unterminated RightScript metadata comment")
	}
	if buffer.Len() == 0 {
		return nil, nil
	}

	err := yaml.Unmarshal(buffer.Bytes(), &metadata)
	if err != nil {
		yamlLineReplace := func(line string) string {
			submatches := yamlLineError.FindStringSubmatch(line)
			number, _ := strconv.ParseUint(submatches[2], 10, 0)
			return fmt.Sprintf(submatches[1]+"line %d:", uint(number)+offset)
		}

		switch err := err.(type) {
		case *yaml.TypeError:
			for index, lineError := range err.Errors {
				err.Errors[index] = yamlLineError.ReplaceAllStringFunc(lineError, yamlLineReplace)
			}
		case error:
			return &metadata, fmt.Errorf(yamlLineError.ReplaceAllStringFunc(err.Error(), yamlLineReplace))
		}
		return &metadata, err
	}
	return &metadata, nil
}

func (metadata *RightScriptMetadata) WriteTo(script io.Writer) (n int64, err error) {
	if metadata.Comment == "" {
		metadata.Comment = "#"
	}

	c, err := fmt.Fprintf(script, "%s ---\n", metadata.Comment)
	if n += int64(c); err != nil {
		return
	}

	yml, err := yaml.Marshal(metadata)
	scanner := bufio.NewScanner(bytes.NewBuffer(yml))

	for scanner.Scan() {
		c, err = fmt.Fprintf(script, "%s %s\n", metadata.Comment, scanner.Text())
		if n += int64(c); err != nil {
			return
		}
	}
	if err = scanner.Err(); err != nil {
		return
	}

	c, err = fmt.Fprintf(script, "%s ...\n", metadata.Comment)
	if n += int64(c); err != nil {
		return
	}

	return
}

func (i InputType) String() string {
	switch i {
	case Single:
		return "single"
	case Array:
		return "array"
	default:
		return fmt.Sprintf("%d", i)
	}
}

func (i InputType) MarshalYAML() (interface{}, error) {
	switch i {
	case Single, Array:
		return i.String(), nil
	default:
		return "", fmt.Errorf("Invalid input type value: %d", i)
	}
}

func (i *InputType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	err := unmarshal(&value)
	if err != nil {
		return err
	}
	switch value {
	case "single":
		*i = Single
	case "array":
		*i = Array
	default:
		return fmt.Errorf("Invalid input type value: %s", value)
	}
	return nil
}

func (i InputValue) String() string {
	return i.Type + ":" + i.Value
}

func (i InputValue) MarshalYAML() (interface{}, error) {
	return i.String(), nil
}

func (i *InputValue) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	err := unmarshal(&value)
	if err != nil {
		return err
	}
	values := strings.SplitN(value, ":", 2)
	if len(values) < 2 {
		return fmt.Errorf("Invalid input value: %s", value)
	}
	*i = InputValue{Type: values[0], Value: values[1]}
	return nil
}
