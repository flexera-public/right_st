package main_test

import (
	"net/http"
	"net/http/httptest"

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
			server              *httptest.Server
			oldUpdateVersionUrl string
		)

		BeforeEach(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`versions:
  1: v1.2.3
  2: v2.3.4
  3: v3.4.5
`))
			}))
			oldUpdateVersionUrl = UpdateVersionUrl
			UpdateVersionUrl = server.URL
		})

		AfterEach(func() {
			UpdateVersionUrl = oldUpdateVersionUrl
			server.Close()
		})

		Describe("Get latest versions", func() {
			It("Gets the latest versions", func() {
				latest, err := UpdateGetLatestVersions()
				Expect(err).NotTo(HaveOccurred())
				Expect(latest).To(Equal(&LatestVersions{
					Versions: map[uint]*Version{
						1: &Version{1, 2, 3},
						2: &Version{2, 3, 4},
						3: &Version{3, 4, 5},
					},
				}))
			})
		})

		Describe("Update check", func() {
			var buffer *gbytes.Buffer

			BeforeEach(func() {
				buffer = gbytes.NewBuffer()
			})

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
`))
			})

			It("Outputs that there is a new major version", func() {
				UpdateCheck("right_st v2.3.4 - JUNK JUNK JUNK", buffer)
				Expect(buffer.Contents()).To(BeEquivalentTo(`There is a new major version of right_st (v3.4.5), to upgrade run:
    right_st update apply -m 3
`))
			})

			It("Outptus that there is a new version and new major version", func() {
				UpdateCheck("right_st v2.0.0 - JUNK JUNK JUNK", buffer)
				Expect(buffer.Contents()).To(BeEquivalentTo(`There is a new v2 version of right_st (v2.3.4), to upgrade run:
    right_st update apply
There is a new major version of right_st (v3.4.5), to upgrade run:
    right_st update apply -m 3
`))
			})
		})
	})
})
