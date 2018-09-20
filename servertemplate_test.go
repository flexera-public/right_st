package main_test

import (
	"io"
	"strings"

	. "github.com/rightscale/right_st"

	"gopkg.in/yaml.v2"
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
  Boot:
    - Name: RL10 Foo
      Revision: 10
      Publisher: RightScale
    - Dummy.sh
  Operational:
    - Dummy2.sh
  Decommission:
    - Dummy3.sh
Alerts:
- Name: CPU Scale Down
  Description: Votes to shrink ServerArray
  Clause: If cpu-0/cpu-idle.value > '50' for 3 minutes Then shrink my_app_name
- Name: Low memory warning
  Clause: If memory/memory-free.value < 100000000 for 5 minutes Then escalate warning
Inputs:
  SERVER_HOSTNAME: text:test.local
MultiCloudImages:
  - Href: /api/multi_cloud_images/403042003
  - Name: FooImage
    Revision: 100
  - Name: FooCorpImage
    Revision: 100
    Publisher: FooCorp
  - Name: ImageBasedMci
    Settings:
    - Cloud: AWS US-West
      Instance Type: t1.micro
      Image: ami-e305efa7
      User Data: Foo
`)
			It("should parse correctly", func() {
				dummy1List := []*RightScript{
					{Type: PublishedRightScript, Name: `RL10 Foo`, Revision: 10, Publisher: `RightScale`},
					{Type: LocalRightScript, Path: "Dummy.sh"},
				}
				dummy2List := []*RightScript{{Type: LocalRightScript, Path: "Dummy2.sh"}}
				dummy3List := []*RightScript{{Type: LocalRightScript, Path: "Dummy3.sh"}}

				st, err := ParseServerTemplate(script)
				Expect(err).To(Succeed())
				Expect(st).NotTo(BeNil())
				Expect(st.Name).To(Equal("Test ST"))
				Expect(st.Description).To(Equal("Test ST Description"))
				Expect(st.Alerts).To(Equal([]*Alert{
					&Alert{Name: "CPU Scale Down",
						Description: "Votes to shrink ServerArray",
						Clause:      "If cpu-0/cpu-idle.value > '50' for 3 minutes Then shrink my_app_name"},
					&Alert{Name: "Low memory warning",
						Clause: "If memory/memory-free.value < 100000000 for 5 minutes Then escalate warning"},
				}))
				Expect(st.Inputs).To(Equal(map[string]*InputValue{"SERVER_HOSTNAME": &InputValue{Type: "text", Value: "test.local"}}))
				Expect(st.RightScripts["Boot"]).To(Equal(dummy1List))
				Expect(st.RightScripts["Operational"]).To(Equal(dummy2List))
				Expect(st.RightScripts["Decommission"]).To(Equal(dummy3List))
				Expect(st.MultiCloudImages).To(Equal([]*MultiCloudImage{
					&MultiCloudImage{Href: "/api/multi_cloud_images/403042003"},
					&MultiCloudImage{Name: "FooImage", Revision: 100},
					&MultiCloudImage{Name: "FooCorpImage", Revision: 100, Publisher: "FooCorp"},
					&MultiCloudImage{Name: "ImageBasedMci", Settings: []*Setting{
						&Setting{Cloud: "AWS US-West", InstanceType: "t1.micro", Image: "ami-e305efa7", UserData: "Foo"},
					}},
				}))
			})
		})

		Context("With invalid structure in YAML", func() {
			It("should return an error", func() {
				script := strings.NewReader(`---
Name: Test ST
Description: Test ST Description
RightScripts:
  Boot:
    - Dummy.sh
Inputs:
  - TEXT_INPUT
# The Inputs field should have a map not an array
`)
				_, err := ParseServerTemplate(script)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(&yaml.TypeError{
					Errors: []string{
						"line 8: cannot unmarshal !!seq into map[string]*main.InputValue",
					},
				}))
			})
		})

		Context("With invalid RightScript type in YAML", func() {
			It("should return an error", func() {
				script := strings.NewReader(`---
Name: Test ST
Description: Test ST Description
RightScripts:
  Unknown:
    - Dummy.sh
Inputs: {}
`)
				_, err := ParseServerTemplate(script)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring("not a valid sequence name"))
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
						"line 4: field Some Bogus Field not found in type main.ServerTemplate",
					},
				}))
			})
		})
	})
})
