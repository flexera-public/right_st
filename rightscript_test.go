package main_test

import (
	. "github.com/rightscale/right_st"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("RightScript", func() {
	DescribeTable("GuessExtension",
		func(source, extension string) {
			Expect(GuessExtension(source)).To(Equal(extension))
		},
		Entry("no shebang", "echo 'Hello, world!'\n", ""),
		Entry("ruby shebang", "#!/usr/bin/ruby\nputs 'Hello, world!'\n", ".rb"),
		Entry("perl shebang", "#!/usr/bin/perl\nprint \"Hello, world!\\n\"\n", ".pl"),
		Entry("bash shebang", "#!/bin/bash\necho 'Hello, world!'\n", ".sh"),
		Entry("PowerShell shebang", "#!PowerShell\necho 'Hello, world!'\n", ".ps1"),
		Entry("PowerShell assignment", "# hello.ps1\n$env:HELLO_WORLD = 'Hello, world!'\necho $env:HELLO_WORLD\n", ".ps1"),
		Entry("PowerShell Write Cmdlets", "# hello.ps1\nWrite-Host 'Hello, world!'\n", ".ps1"),
	)
})
