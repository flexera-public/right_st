package main_test

import (
	"net/http"
	"net/http/httptest"
	"runtime"

	. "github.com/rightscale/right_st"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Update", func() {
	Describe("Get current version", func() {
		It("Gets a version from a tagged version", func() {
			version := UpdateGetCurrentVersion("right_st v98.76.54 - JUNK JUNK JUNK")
			Expect(version).To(Equal(&Version{98, 76, 54}))
		})

		It("Gets no version for a dev version", func() {
			version := UpdateGetCurrentVersion("right_st master - JUNK JUNK JUNK")
			Expect(version).To(BeNil())
		})
	})

	Context("With a update versions URL", func() {
		var (
			buffer           *gbytes.Buffer
			server           *httptest.Server
			oldUpdateBaseUrl string
		)

		BeforeEach(func() {
			buffer = gbytes.NewBuffer()
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/version.yml":
					w.Write([]byte(`# Latest right_st versions by major version (this file is used by right_st's update check mechanism)
---
versions:
  1: v1.2.3
  2: v2.3.4
  3: v3.4.5
`))
				}
			}))
			oldUpdateBaseUrl = UpdateBaseUrl
			UpdateBaseUrl = server.URL
		})

		AfterEach(func() {
			UpdateBaseUrl = oldUpdateBaseUrl
			server.Close()
		})

		Describe("Get latest versions", func() {
			It("Gets the latest versions", func() {
				latest, err := UpdateGetLatestVersions()
				Expect(err).NotTo(HaveOccurred())
				Expect(latest).To(Equal(&LatestVersions{
					Versions: map[int]*Version{
						1: &Version{1, 2, 3},
						2: &Version{2, 3, 4},
						3: &Version{3, 4, 5},
					},
				}))
			})
		})

		Describe("Get download URL", func() {
			var ext string
			if runtime.GOOS == "windows" {
				ext = "zip"
			} else {
				ext = "tgz"
			}

			It("Gets the download URL for a major version", func() {
				url, version, err := UpdateGetDownloadUrl(1)
				Expect(err).NotTo(HaveOccurred())
				Expect(url).To(Equal(server.URL + "/v1.2.3/right_st-" + runtime.GOOS + "-" + runtime.GOARCH + "." + ext))
				Expect(version).To(Equal(&Version{1, 2, 3}))
			})

			It("Returns an error for a nonexistent major version", func() {
				url, version, err := UpdateGetDownloadUrl(0)
				Expect(err).To(MatchError("Major version not available: 0"))
				Expect(url).To(BeEmpty())
				Expect(version).To(BeNil())
			})
		})

		Describe("Update check", func() {
			It("Outputs nothing for a dev version", func() {
				UpdateCheck("right_st dev - JUNK JUNK JUNK", buffer)
				Expect(buffer.Contents()).To(BeEmpty())
			})

			It("Outputs nothing if there is no update", func() {
				UpdateCheck("right_st v3.4.5 - JUNK JUNK JUNK", buffer)
				Expect(buffer.Contents()).To(BeEmpty())
			})

			It("Outputs that there is a new version", func() {
				UpdateCheck("right_st v3.0.0 - JUNK JUNK JUNK", buffer)
				Expect(buffer.Contents()).To(BeEquivalentTo(`There is a new v3 version of right_st (v3.4.5), to upgrade run:
    right_st update apply

See https://github.com/rightscale/right_st/blob/master/ChangeLog.md or
https://github.com/rightscale/right_st/releases for more information.
`))
			})

			It("Outputs that there is a new major version", func() {
				UpdateCheck("right_st v2.3.4 - JUNK JUNK JUNK", buffer)
				Expect(buffer.Contents()).To(BeEquivalentTo(`There is a new major version of right_st (v3.4.5), to upgrade run:
    right_st update apply -m 3

See https://github.com/rightscale/right_st/blob/master/ChangeLog.md or
https://github.com/rightscale/right_st/releases for more information.
`))
			})

			It("Outptus that there is a new version and new major version", func() {
				UpdateCheck("right_st v2.0.0 - JUNK JUNK JUNK", buffer)
				Expect(buffer.Contents()).To(BeEquivalentTo(`There is a new v2 version of right_st (v2.3.4), to upgrade run:
    right_st update apply
There is a new major version of right_st (v3.4.5), to upgrade run:
    right_st update apply -m 3

See https://github.com/rightscale/right_st/blob/master/ChangeLog.md or
https://github.com/rightscale/right_st/releases for more information.
`))
			})
		})

		Describe("Update list", func() {
			It("Outputs the available versions for a dev version", func() {
				err := UpdateList("right_st dev - JUNK JUNK JUNK", buffer)
				Expect(err).NotTo(HaveOccurred())
				Expect(buffer.Contents()).To(BeEquivalentTo(`The latest v1 version of right_st is v1.2.3.
The latest v2 version of right_st is v2.3.4.
The latest v3 version of right_st is v3.4.5.

See https://github.com/rightscale/right_st/blob/master/ChangeLog.md or
https://github.com/rightscale/right_st/releases for more information.
`))
			})

			It("Outputs the available versions for an up to date version", func() {
				err := UpdateList("right_st v3.4.5 - JUNK JUNK JUNK", buffer)
				Expect(err).NotTo(HaveOccurred())
				Expect(buffer.Contents()).To(BeEquivalentTo(`The latest v1 version of right_st is v1.2.3.
The latest v2 version of right_st is v2.3.4.
The latest v3 version of right_st is v3.4.5; this is the version you are using!

See https://github.com/rightscale/right_st/blob/master/ChangeLog.md or
https://github.com/rightscale/right_st/releases for more information.
`))
			})

			It("Outputs the available versions when there is a new version", func() {
				err := UpdateList("right_st v3.0.0 - JUNK JUNK JUNK", buffer)
				Expect(err).NotTo(HaveOccurred())
				Expect(buffer.Contents()).To(BeEquivalentTo(`The latest v1 version of right_st is v1.2.3.
The latest v2 version of right_st is v2.3.4.
The latest v3 version of right_st is v3.4.5; you are using v3.0.0, to upgrade run:
    right_st update apply

See https://github.com/rightscale/right_st/blob/master/ChangeLog.md or
https://github.com/rightscale/right_st/releases for more information.
`))
			})

			It("Outputs the available versions when there is a new major version", func() {
				err := UpdateList("right_st v2.3.4 - JUNK JUNK JUNK", buffer)
				Expect(err).NotTo(HaveOccurred())
				Expect(buffer.Contents()).To(BeEquivalentTo(`The latest v1 version of right_st is v1.2.3.
The latest v2 version of right_st is v2.3.4; this is the version you are using!
The latest v3 version of right_st is v3.4.5; you are using v2.3.4, to upgrade run:
    right_st update apply -m 3

See https://github.com/rightscale/right_st/blob/master/ChangeLog.md or
https://github.com/rightscale/right_st/releases for more information.
`))
			})

			It("Outputs the available versions when there is a new version and new major version", func() {
				err := UpdateList("right_st v2.0.0 - JUNK JUNK JUNK", buffer)
				Expect(err).NotTo(HaveOccurred())
				Expect(buffer.Contents()).To(BeEquivalentTo(`The latest v1 version of right_st is v1.2.3.
The latest v2 version of right_st is v2.3.4; you are using v2.0.0, to upgrade run:
    right_st update apply
The latest v3 version of right_st is v3.4.5; you are using v2.0.0, to upgrade run:
    right_st update apply -m 3

See https://github.com/rightscale/right_st/blob/master/ChangeLog.md or
https://github.com/rightscale/right_st/releases for more information.
`))
			})
		})
	})
})
