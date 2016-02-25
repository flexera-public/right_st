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
	Name        string    `yaml:"RightScript Name"`
	Description string    `yaml:"Description"`
	Inputs      *InputMap `yaml:"Inputs"`
	Attachments []string  `yaml:"Attachments"`
	Comment     string    `yaml:"-"`
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

type InputMap map[string]*InputMetadata

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

	if metadata.Inputs == nil {
		inputs := make(InputMap)
		metadata.Inputs = &inputs
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
	inputType, err := parseInputType(value)
	if err != nil {
		return fmt.Errorf("Invalid input type value: %s", value)
	}
	*i = inputType
	return nil
}

func parseInputType(value string) (InputType, error) {
	switch value {
	case "single":
		return Single, nil
	case "array":
		return Array, nil
	default:
		return Single, fmt.Errorf("Invalid input type value: %s", value)
	}
}

func (i InputValue) String() string {
	switch i.Type {
	case "blank", "ignore":
		return i.Type
	default:
		return i.Type + ":" + i.Value
	}
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

	iv_pnt, err := parseInputValue(value)
	if err != nil {
		return err
	}
	*i = *iv_pnt
	return nil
}

func parseInputValue(value string) (*InputValue, error) {
	values := strings.SplitN(value, ":", 2)
	switch values[0] {
	case "blank", "ignore":
		return &InputValue{Type: values[0]}, nil
	default:
		if len(values) < 2 {
			return nil, fmt.Errorf("Invalid input value: %s", value)
		}
		i := InputValue{Type: values[0], Value: values[1]}
		if i.Type == "text" && i.Value == "" {
			return nil, fmt.Errorf("Use 'blank' or 'ignore' instead of 'text:'")
		}
		return &i, nil
	}
}
