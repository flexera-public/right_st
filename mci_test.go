package main_test

import (
	. "github.com/rightscale/right_st"
	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("MultiCloudImage", func() {
	DescribeTable("MarshalYAML",
		func(mci MultiCloudImage, source string) {
			Expect(yaml.Marshal(&mci)).To(MatchYAML(source))
		},
		Entry("Name, Revision, and Publisher",
			MultiCloudImage{
				Name:      "Ubuntu 19.10",
				Revision:  1,
				Publisher: "RightScale",
			}, `Name: Ubuntu 19.10
Revision: 1
Publisher: RightScale
`),
		Entry("Name and Revision",
			MultiCloudImage{
				Name:     "Ubuntu 20.04",
				Revision: 0,
			}, `Name: Ubuntu 20.04
Revision: head
`),
		Entry("Href",
			MultiCloudImage{
				Href: "/api/multi_cloud_images/123456789",
			}, `Href: /api/multi_cloud_images/123456789
`),
		Entry("Fully specified",
			MultiCloudImage{
				Name: "Amazon Linux HVM EBS-Backed 64-bit",
				Description: `Amazon Linux 2 HVM EBS-Backed 64-bit
====================================

` + "`" + `amzn2-ami-hvm-2.0.20190618-x86_64-ebs` + "`" + ` [Release Notes](https://aws.amazon.com/amazon-linux-2/release-notes/)
`,
				Tags: []string{
					"rs_agent:type=right_link_lite",
					"rs_agent:mime_shellscript=https://rightlink.rightscale.com/rll/10/rightlink.boot.sh",
				},
				Settings: []*Setting{
					{
						Cloud:        "AWS US-East",
						InstanceType: "t2.micro",
						Image:        "ami-00b882ac5193044e4",
					},
				},
			}, `Name: Amazon Linux HVM EBS-Backed 64-bit
Description: |
  Amazon Linux 2 HVM EBS-Backed 64-bit
  ====================================

  `+"`"+`amzn2-ami-hvm-2.0.20190618-x86_64-ebs`+"`"+` [Release Notes](https://aws.amazon.com/amazon-linux-2/release-notes/)
Tags:
  - rs_agent:type=right_link_lite
  - rs_agent:mime_shellscript=https://rightlink.rightscale.com/rll/10/rightlink.boot.sh
Settings:
  - Cloud: AWS US-East
    Instance Type: t2.micro
    Image: ami-00b882ac5193044e4
`),
	)
})
