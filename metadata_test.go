package main_test

import (
	"io"
	"strings"

	. "github.com/douglaswth/right_st"

	"github.com/go-yaml/yaml"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("RightScript Metadata", func() {
	var (
		validScript                     io.ReadSeeker
		noMetadataScript                io.ReadSeeker
		missingEndDelimiterScript       io.ReadSeeker
		invalidYamlSyntaxScript         io.ReadSeeker
		invalidMetadataStructureScript  io.ReadSeeker
		incorrectInputTypeSyntaxScript  io.ReadSeeker
		incorrectInputValueSyntaxScript io.ReadSeeker
		emptyTextValueScript            io.ReadSeeker
		unknownFieldScript              io.ReadSeeker
		buffer                          *gbytes.Buffer
		emptyMetadata                   RightScriptMetadata
		emptyMetadataScript             string
		populatedMetadata               RightScriptMetadata
		populatedMetadataScript         string
		differentCommentMetadata        RightScriptMetadata
		differentCommentMetadataScript  string
	)

	BeforeEach(func() {
		validScript = strings.NewReader(`#!/bin/bash
# ---
# RightScript Name: Some RightScript Name
# Description: Some description of stuff
# Inputs:
#   TEXT_INPUT:
#     Category: Uncategorized
#     Description: Some test input
#     Input Type: single
#     Required: true
#     Advanced: false
#     Default: text:foobar
#     Possible Values:
#       - text:foobar
#       - text:barfoo
#   SUPPORTED_VERSIONS:
#     Category: Uncategorized
#     Description: Some array input
#     Input Type: array
#     Required: false
#     Advanced: true
#     Default: array:["text:v1","text:v2"]
# Attachments:
#   - attachments/some_attachment.zip
#   - attachments/another_attachment.tar.xz
# ...
`)
		noMetadataScript = strings.NewReader(`#!/bin/bash
# There is no metadata comment here
`)
		missingEndDelimiterScript = strings.NewReader(`#!/bin/bash
# ---
# RightScript Name: Some RightScript Name
# Description: Some description of stuff
# Inputs:
#   TEXT_INPUT:
#     Category: Uncategorized
#     Description: Some test input
#     Input Type: single
#     Required: true
#     Advanced: false
#     Default: text:foobar
#     Possible Values:
#       - text:foobar
#       - text:barfoo
#
# We should have used the '...' end delimiter above
`)
		invalidYamlSyntaxScript = strings.NewReader(`#!/bin/bash
# ---
# RightScript Name: Some RightScript Name
# Description: Some description of stuff
# Inputs:
#   TEXT_INPUT:
#     Category: Uncategorized
#     Description: Some test input
#     Input Type: bogus
#     Required: true
#     Advanced: false
#     Default: text:
# ...
# The Default line is invalid YAML
`)
		invalidMetadataStructureScript = strings.NewReader(`#!/bin/bash
# ---
# RightScript Name: Some RightScript Name
# Description: Some description of stuff
# Inputs:
#   - TEXT_INPUT
# ...
# The Inputs field should have a map not an array
`)
		incorrectInputTypeSyntaxScript = strings.NewReader(`#!/bin/bash
# ---
# RightScript Name: Some RightScript Name
# Description: Some description of stuff
# Inputs:
#   TEXT_INPUT:
#     Category: Uncategorized
#     Description: Some test input
#     Input Type: bogus
#     Required: true
#     Advanced: false
# ...
# The Input Type line is not a valid type
`)
		incorrectInputValueSyntaxScript = strings.NewReader(`#!/bin/bash
# ---
# RightScript Name: Some RightScript Name
# Description: Some description of stuff
# Inputs:
#   TEXT_INPUT:
#     Category: Uncategorized
#     Description: Some test input
#     Input Type: single
#     Required: true
#     Advanced: false
#     Default: foobar
# ...
# The Default line is not valid Inputs 2.0 syntax
`)
		emptyTextValueScript = strings.NewReader(`#!/bin/bash
# ---
# RightScript Name: Some RightScript Name
# Description: Some description of stuff
# Inputs:
#   TEXT_INPUT:
#     Category: Uncategorized
#     Description: Some test input
#     Input Type: single
#     Required: true
#     Advanced: false
#     Default: "text:"
# ...
# The Default line should be blank or ignore in Inputs 2.0 syntax
`)
		unknownFieldScript = strings.NewReader(`#!/bin/bash
# ---
# RightScript Name: Some RightScript Name
# Description: Some description of stuff
# Some Bogus Field: Some bogus value
# ...
`)
		buffer = gbytes.NewBuffer()
		emptyMetadata = RightScriptMetadata{}
		emptyMetadataScript = `# ---
# RightScript Name: ""
# Description: ""
# Inputs: {}
# Attachments: []
# ...
`
		populatedMetadata = RightScriptMetadata{
			Name:        "Some RightScript Name",
			Description: "Some description of stuff",
			Inputs: &InputMap{
				"TEXT_INPUT": {
					Category:       "Uncategorized",
					Description:    "Some test input",
					InputType:      0,
					Required:       true,
					Advanced:       false,
					Default:        &InputValue{"text", "foobar"},
					PossibleValues: []*InputValue{&InputValue{"text", "foobar"}, &InputValue{"text", "barfoo"}},
				},
				"SUPPORTED_VERSIONS": {
					Category:    "Uncategorized",
					Description: "Some array input",
					InputType:   1,
					Required:    false,
					Advanced:    true,
					Default:     &InputValue{"array", `["text:v1","text:v2"]`},
				},
			},
			Attachments: []string{
				"attachments/some_attachment.zip",
				"attachments/another_attachment.tar.xz",
			},
		}
		populatedMetadataScript = `# ---
# RightScript Name: Some RightScript Name
# Description: Some description of stuff
# Inputs:
#   SUPPORTED_VERSIONS:
#     Category: Uncategorized
#     Description: Some array input
#     Input Type: array
#     Required: false
#     Advanced: true
#     Default: array:["text:v1","text:v2"]
#   TEXT_INPUT:
#     Category: Uncategorized
#     Description: Some test input
#     Input Type: single
#     Required: true
#     Advanced: false
#     Default: text:foobar
#     Possible Values:
#     - text:foobar
#     - text:barfoo
# Attachments:
# - attachments/some_attachment.zip
# - attachments/another_attachment.tar.xz
# ...
`
		differentCommentMetadata = RightScriptMetadata{Comment: "//"}
		differentCommentMetadataScript = `// ---
// RightScript Name: ""
// Description: ""
// Inputs: {}
// Attachments: []
// ...
`
	})

	Describe("Parse RightScript metadata", func() {
		Context("With valid script metadata", func() {
			It("should parse correctly", func() {
				metadata, err := ParseRightScriptMetadata(validScript)
				Expect(err).To(Succeed())
				Expect(metadata).NotTo(BeNil())
				Expect(metadata.Name).To(Equal("Some RightScript Name"))
				Expect(metadata.Description).To(Equal("Some description of stuff"))
				Expect(metadata.Inputs).To(Equal(&InputMap{
					"TEXT_INPUT": {
						Category:       "Uncategorized",
						Description:    "Some test input",
						InputType:      0,
						Required:       true,
						Advanced:       false,
						Default:        &InputValue{"text", "foobar"},
						PossibleValues: []*InputValue{&InputValue{"text", "foobar"}, &InputValue{"text", "barfoo"}},
					},
					"SUPPORTED_VERSIONS": {
						Category:    "Uncategorized",
						Description: "Some array input",
						InputType:   1,
						Required:    false,
						Advanced:    true,
						Default:     &InputValue{"array", `["text:v1","text:v2"]`},
					},
				}))
				Expect(metadata.Attachments).To(Equal([]string{
					"attachments/some_attachment.zip",
					"attachments/another_attachment.tar.xz",
				}))
			})
		})

		Context("With no script metadata", func() {
			It("should not return metadata", func() {
				metadata, err := ParseRightScriptMetadata(noMetadataScript)
				Expect(err).To(Succeed())
				Expect(metadata).To(BeNil())
			})
		})

		Context("With missing end delimiter in script metadata", func() {
			It("should return an error", func() {
				_, err := ParseRightScriptMetadata(missingEndDelimiterScript)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("Unterminated RightScript metadata comment"))
			})
		})

		Context("With invalid YAML syntax in script metadata", func() {
			It("should return an error", func() {
				_, err := ParseRightScriptMetadata(invalidYamlSyntaxScript)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("yaml: line 12: mapping values are not allowed in this context"))
			})
		})

		Context("With invalid structure in script metadata", func() {
			It("should return an error", func() {
				_, err := ParseRightScriptMetadata(invalidMetadataStructureScript)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(&yaml.TypeError{
					Errors: []string{
						"line 7: cannot unmarshal !!seq into main.InputMap",
					},
				}))
			})
		})

		Context("With incorrect input type syntax in script metadata", func() {
			It("should return an error", func() {
				_, err := ParseRightScriptMetadata(incorrectInputTypeSyntaxScript)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("Invalid input type value: bogus"))
			})
		})

		Context("With incorrect input value syntax in script metadata", func() {
			It("should return an error", func() {
				_, err := ParseRightScriptMetadata(incorrectInputValueSyntaxScript)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("Invalid input value: foobar"))
			})
		})

		Context("With a blank text input value in script metadata", func() {
			It("should return an error", func() {
				_, err := ParseRightScriptMetadata(emptyTextValueScript)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("Use 'blank' or 'ignore' instead of 'text:'"))
			})
		})

		Context("With an unknown field in script metadata", func() {
			It("should return an error", func() {
				_, err := ParseRightScriptMetadata(unknownFieldScript)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(&yaml.TypeError{
					Errors: []string{
						"line 4: no such field 'Some Bogus Field' in struct 'main.RightScriptMetadata'",
					},
				}))
			})
		})
	})

	Describe("Write RightScript metadata", func() {
		Context("With empty metadata", func() {
			It("should write a metadata comment", func() {
				n, err := emptyMetadata.WriteTo(buffer)
				Expect(err).To(Succeed())
				Expect(buffer.Contents()).To(BeEquivalentTo(emptyMetadataScript))
				Expect(n).To(BeEquivalentTo(84))
			})
		})
		Context("With populated metadata", func() {
			It("should write a metadata comment", func() {
				n, err := populatedMetadata.WriteTo(buffer)
				Expect(err).To(Succeed())
				Expect(buffer.Contents()).To(BeEquivalentTo(populatedMetadataScript))
				Expect(n).To(BeEquivalentTo(637))
			})
		})
		Context("With a different comment string for metadata", func() {
			It("should write a metadata comment", func() {
				n, err := differentCommentMetadata.WriteTo(buffer)
				Expect(err).To(Succeed())
				Expect(buffer.Contents()).To(BeEquivalentTo(differentCommentMetadataScript))
				Expect(n).To(BeEquivalentTo(90))
			})
		})
	})
})
