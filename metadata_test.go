package main_test

import (
	"io"
	"strings"

	. "github.com/rightscale/right_st"

	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("RightScript Metadata", func() {
	Describe("Parse RightScript metadata", func() {
		Context("With valid script metadata", func() {
			var validScript io.ReadSeeker
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

			It("should parse correctly", func() {
				metadata, err := ParseRightScriptMetadata(validScript)
				Expect(err).To(Succeed())
				Expect(metadata).NotTo(BeNil())
				Expect(metadata.Name).To(Equal("Some RightScript Name"))
				Expect(metadata.Description).To(Equal("Some description of stuff"))
				Expect(metadata.Inputs).To(Equal(InputMap{
					InputMetadata{
						Name:           "TEXT_INPUT",
						Category:       "Uncategorized",
						Description:    "Some test input",
						InputType:      0,
						Required:       true,
						Advanced:       false,
						Default:        &InputValue{"text", "foobar"},
						PossibleValues: []*InputValue{&InputValue{"text", "foobar"}, &InputValue{"text", "barfoo"}},
					},
					InputMetadata{
						Name:        "SUPPORTED_VERSIONS",
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
			var noMetadataScript io.ReadSeeker
			noMetadataScript = strings.NewReader(`#!/bin/bash
# There is no metadata comment here
`)

			It("should not return metadata", func() {
				metadata, err := ParseRightScriptMetadata(noMetadataScript)
				Expect(err).To(Succeed())
				Expect(metadata).To(BeNil())
			})
		})

		Context("With missing end delimiter in script metadata", func() {
			var missingEndDelimiterScript io.ReadSeeker
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

			It("should return an error", func() {
				_, err := ParseRightScriptMetadata(missingEndDelimiterScript)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("Unterminated RightScript metadata comment"))
			})
		})

		Context("With invalid YAML syntax in script metadata", func() {
			var invalidYamlSyntaxScript io.ReadSeeker
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

			It("should return an error", func() {
				_, err := ParseRightScriptMetadata(invalidYamlSyntaxScript)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("yaml: line 12: mapping values are not allowed in this context"))
			})
		})

		Context("With invalid structure in script metadata", func() {
			var invalidMetadataStructureScript io.ReadSeeker
			invalidMetadataStructureScript = strings.NewReader(`#!/bin/bash
# ---
# RightScript Name: Some RightScript Name
# Description: Some description of stuff
# Inputs:
#   - TEXT_INPUT
# ...
# The Inputs field should have a map not an array
`)

			It("should return an error", func() {
				_, err := ParseRightScriptMetadata(invalidMetadataStructureScript)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(&yaml.TypeError{
					Errors: []string{
						"line 6: cannot unmarshal !!seq into map[string]main.InputMetadata",
					},
				}))
			})
		})

		Context("With incorrect input type syntax in script metadata", func() {
			var incorrectInputTypeSyntaxScript io.ReadSeeker
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

			It("should return an error", func() {
				_, err := ParseRightScriptMetadata(incorrectInputTypeSyntaxScript)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("Invalid input type value: bogus"))
			})
		})

		Context("With incorrect input value syntax in script metadata", func() {
			var incorrectInputValueSyntaxScript io.ReadSeeker
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

			It("should return an error", func() {
				_, err := ParseRightScriptMetadata(incorrectInputValueSyntaxScript)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("Invalid input value: foobar"))
			})
		})

		Context("With a blank text input value in script metadata", func() {
			var emptyTextValueScript io.ReadSeeker
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

			It("should return an error", func() {
				_, err := ParseRightScriptMetadata(emptyTextValueScript)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("Use 'blank' or 'ignore' instead of 'text:'"))
			})
		})

		Context("With an unknown field in script metadata", func() {
			var unknownFieldScript io.ReadSeeker
			unknownFieldScript = strings.NewReader(`#!/bin/bash
# ---
# RightScript Name: Some RightScript Name
# Description: Some description of stuff
# Some Bogus Field: Some bogus value
# ...
`)

			It("should return an error", func() {
				_, err := ParseRightScriptMetadata(unknownFieldScript)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(&yaml.TypeError{
					Errors: []string{
						"line 5: field Some Bogus Field not found in type main.RightScriptMetadata",
					},
				}))
			})
		})

		Context("With a duplicate input in script metadata", func() {
			var duplicateInputScript io.ReadSeeker
			duplicateInputScript = strings.NewReader(`#!/bin/bash
# ---
# RightScript Name: Some RightScript Name
# Description: Some description of stuff
# Inputs:
#  DUPLICATE_INPUT:
#    Category: Uncategorized
#    Description: The first duplicate input
#    Input Type: single
#    Required: true
#    Advanced: false
#  DUPLICATE_INPUT:
#    Category: Uncategorized
#    Description: The second duplicate input
#    Input Type: single
#    Required: true
#    Advanced: false
# ...
`)

			It("should return an error", func() {
				_, err := ParseRightScriptMetadata(duplicateInputScript)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(&yaml.TypeError{
					Errors: []string{
						`line 13: key "DUPLICATE_INPUT" already set in map`,
					},
				}))
			})
		})
	})

	Describe("Write RightScript metadata", func() {
		var buffer *gbytes.Buffer

		BeforeEach(func() {
			buffer = gbytes.NewBuffer()
		})

		Context("With empty metadata", func() {
			var (
				emptyMetadata       RightScriptMetadata
				emptyMetadataScript string
			)
			emptyMetadata = RightScriptMetadata{}
			emptyMetadataScript = `# ---
# RightScript Name: ""
# Inputs: {}
# Attachments: []
# ...
`

			It("should write a metadata comment", func() {
				n, err := emptyMetadata.WriteTo(buffer)
				Expect(err).To(Succeed())
				Expect(buffer.Contents()).To(BeEquivalentTo(emptyMetadataScript))
				Expect(n).To(BeEquivalentTo(66))
			})
		})

		Context("With populated metadata", func() {
			var (
				populatedMetadata       RightScriptMetadata
				populatedMetadataScript string
			)
			populatedMetadata = RightScriptMetadata{
				Name:        "Some RightScript Name",
				Description: "Some description of stuff",
				Inputs: InputMap{
					InputMetadata{
						Name:           "TEXT_INPUT",
						Category:       "Uncategorized",
						Description:    "Some test input",
						InputType:      0,
						Required:       true,
						Advanced:       false,
						Default:        &InputValue{"text", "foobar"},
						PossibleValues: []*InputValue{&InputValue{"text", "foobar"}, &InputValue{"text", "barfoo"}},
					},
					InputMetadata{
						Name:        "SUPPORTED_VERSIONS",
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
#   SUPPORTED_VERSIONS:
#     Category: Uncategorized
#     Description: Some array input
#     Input Type: array
#     Required: false
#     Advanced: true
#     Default: array:["text:v1","text:v2"]
# Attachments:
# - attachments/some_attachment.zip
# - attachments/another_attachment.tar.xz
# ...
`

			It("should write a metadata comment", func() {
				n, err := populatedMetadata.WriteTo(buffer)
				Expect(err).To(Succeed())
				Expect(buffer.Contents()).To(BeEquivalentTo(populatedMetadataScript))
				Expect(n).To(BeEquivalentTo(637))
			})
		})

		Context("With a different comment string for metadata", func() {
			var (
				differentCommentMetadata       RightScriptMetadata
				differentCommentMetadataScript string
			)
			differentCommentMetadata = RightScriptMetadata{Comment: "//"}
			differentCommentMetadataScript = `// ---
// RightScript Name: ""
// Inputs: {}
// Attachments: []
// ...
`

			It("should write a metadata comment", func() {
				n, err := differentCommentMetadata.WriteTo(buffer)
				Expect(err).To(Succeed())
				Expect(buffer.Contents()).To(BeEquivalentTo(differentCommentMetadataScript))
				Expect(n).To(BeEquivalentTo(71))
			})
		})
	})
})
