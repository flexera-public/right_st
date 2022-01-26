package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
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
	Name        string   `yaml:"RightScript Name"`
	Description string   `yaml:"Description,omitempty"`
	Packages    string   `yaml:"Packages,omitempty"`
	Inputs      InputMap `yaml:"Inputs"`
	Attachments []string `yaml:"Attachments"`
	Comment     string   `yaml:"-"`
}

type InputMetadata struct {
	Name           string        `yaml:"-"`
	Category       string        `yaml:"Category"`
	Description    string        `yaml:"Description,omitempty"`
	InputType      InputType     `yaml:"Input Type"`
	Required       bool          `yaml:"Required"`
	Advanced       bool          `yaml:"Advanced"`
	Default        *InputValue   `yaml:"Default,omitempty"`
	PossibleValues []*InputValue `yaml:"Possible Values,omitempty"`
}

// We use an array internally but serialize to a YAML map by performing transformations on this.
// See UnmarshalYAML/MarshalYAML functions for details
type InputMap []InputMetadata

type InputType int

type InputValue struct {
	Type  string
	Value string
}

func ParseRightScriptMetadata(script io.ReadSeeker) (*RightScriptMetadata, error) {
	defer func() {
		_, _ = script.Seek(0, io.SeekStart)
	}()

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
			offset = lineNumber - 1
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

	err := yaml.UnmarshalStrict(buffer.Bytes(), &metadata)
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
		metadata.Inputs = InputMap{}
	}

	c, err := fmt.Fprintf(script, "%s ---\n", metadata.Comment)
	if n += int64(c); err != nil {
		return
	}

	yml, err := yaml.Marshal(metadata)
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(bytes.NewBuffer(yml))

	for scanner.Scan() {
		t := scanner.Text()
		if len(t) > 0 {
			c, err = fmt.Fprintf(script, "%s %s\n", metadata.Comment, t)
		} else {
			c, err = fmt.Fprintf(script, "%s\n", metadata.Comment)
		}
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

// The Marshal/Unmarshal for InputMap bears some explanation. So we have a simple hash/map defined as InputMap in the
// publicly facing Input Definition. Hash/maps in golang however aren't ordered. Luckily yaml.v2 for golang has an
// ordered Hash called MapSlice, which is an array of MapItems which have a Key and Value field. So we turn our internal
// array into this MapSlice when Marshalling to get something that looks like a hash but is ordered. When Unmarshalling,
// we unmarshal to BOTH a MapSlice and a Hash of InputDefinitions. The MapSlice is simply mined to get the order.
// The hash of InputDefinitions is so we can make sure we unmarshal into a well ordered struct instead of the untyped
// interface{} pile that is MapSlice. Magic!
func (im InputMap) MarshalYAML() (interface{}, error) {
	// with the API, internally we store Inputs as an array of values.
	destMap := yaml.MapSlice{}
	for _, i := range im {
		destMap = append(destMap, yaml.MapItem{Key: i.Name, Value: i})
	}
	return destMap, nil
}

func (im *InputMap) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var valueOrder yaml.MapSlice
	var value map[string]InputMetadata
	err := unmarshal(&value) // For convenient strong typed-ness
	if err != nil {
		return err
	}
	err = unmarshal(&valueOrder) // For ordering information
	if err != nil {
		return err
	}
	inputMap := InputMap{}
	for _, v := range valueOrder {
		inputName := v.Key.(string)

		inputMetadata := value[inputName]
		inputMetadata.Name = inputName
		inputMap = append(inputMap, inputMetadata)
	}

	*im = inputMap
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
