package main_test

import (
	"io"
	"strings"

	. "github.com/douglaswth/right_st"

	"github.com/go-yaml/yaml"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServerTemplate", func() {
	var script io.Reader

	Describe("Parse", func() {
		Context("With valid YAML", func() {
			script = strings.NewReader(`---
Name: Test ST
Description: Test ST Description
RightScripts:
  - Dummy.sh
Inputs:
  SERVER_HOSTNAME:
    Input Type: single
    Category: RightScale
    Description: Sets the hostname
    Default: "text:test.local"
    Required: false
    Advanced: true
MultiCloudImages:
  - Href: /api/multi_cloud_images/403042003
`)
			It("should parse correctly", func() {
				st, err := ParseServerTemplate(script)
				Expect(err).To(Succeed())
				Expect(st).NotTo(BeNil())
				Expect(st.Name).To(Equal("Test ST"))
				Expect(st.Description).To(Equal("Test ST Description"))
				Expect(st.Inputs).To(Equal(map[string]*InputMetadata{
					"SERVER_HOSTNAME": {
						Category:    "RightScale",
						Description: "Sets the hostname",
						InputType:   0,
						Required:    false,
						Advanced:    true,
						Default:     &InputValue{"text", "test.local"},
					},
				}))
				Expect(st.RightScripts).To(Equal([]string{
					"Dummy.sh",
				}))
				Expect(st.MultiCloudImages).To(Equal([]*Image{
					&Image{Href: "/api/multi_cloud_images/403042003"},
				}))
			})
		})

		Context("With invalid structure in YAML", func() {
			It("should return an error", func() {
				script := strings.NewReader(`---
Name: Test ST
Description: Test ST Description
RightScripts:
  - Dummy.sh
Inputs:
  - TEXT_INPUT
# The Inputs field should have a map not an array
`)
				_, err := ParseServerTemplate(script)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(&yaml.TypeError{
					Errors: []string{
						"line 7: cannot unmarshal !!seq into map[string]*main.InputMetadata",
					},
				}))
			})
		})

		Context("With an unknown field in YAML", func() {
			It("should return an error", func() {
				script := strings.NewReader(`---
Name: Test ST
Description: Test ST Description
Some Bogus Field: Some bogus value
`)
				_, err := ParseServerTemplate(script)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(&yaml.TypeError{
					Errors: []string{
						"line 2: no such field 'Some Bogus Field' in struct 'main.ServerTemplate'",
					},
				}))
			})
		})
	})
})
