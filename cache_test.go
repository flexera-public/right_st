package main_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/rightscale/right_st"
)

var _ = Describe("Cache", func() {
	var (
		tempPath string
		cache    Cache
	)

	BeforeEach(func() {
		var err error
		tempPath, err = ioutil.TempDir("", "test.right_st.cache.")
		if err != nil {
			panic(err)
		}
		cache, err = NewCache(tempPath)
		if err != nil {
			panic(err)
		}
	})

	AfterEach(func() {
		err := os.RemoveAll(tempPath)
		if err != nil {
			panic(err)
		}
	})

	Describe("GetServerTemplateFile", func() {
		It("gets a cached ServerTemplate file path", func() {
			path, err := cache.GetServerTemplateFile(1, "2345678", 90)
			Expect(err).NotTo(HaveOccurred())
			Expect(filepath.Base(path)).To(Equal("server_template.yml"))

			revisionPath := filepath.Dir(path)
			Expect(filepath.Base(revisionPath)).To(Equal("90"))
			Expect(revisionPath).To(BeADirectory())

			idPath := filepath.Dir(revisionPath)
			Expect(filepath.Base(idPath)).To(Equal("2345678"))
			Expect(idPath).To(BeADirectory())

			accountPath := filepath.Dir(idPath)
			Expect(filepath.Base(accountPath)).To(Equal("1"))
			Expect(accountPath).To(BeADirectory())

			typePath := filepath.Dir(accountPath)
			Expect(filepath.Base(typePath)).To(Equal("server_templates"))
			Expect(typePath).To(BeADirectory())

			cachePath := filepath.Dir(typePath)
			Expect(cachePath).To(Equal(tempPath))
		})
	})

	Describe("GetRightScriptFile", func() {
		It("gets a cached RightScript file path", func() {
			path, err := cache.GetRightScriptFile(1, "2345678", 90)
			Expect(err).NotTo(HaveOccurred())
			Expect(filepath.Base(path)).To(Equal("right_script"))

			revisionPath := filepath.Dir(path)
			Expect(filepath.Base(revisionPath)).To(Equal("90"))
			Expect(revisionPath).To(BeADirectory())

			idPath := filepath.Dir(revisionPath)
			Expect(filepath.Base(idPath)).To(Equal("2345678"))
			Expect(idPath).To(BeADirectory())

			accountPath := filepath.Dir(idPath)
			Expect(filepath.Base(accountPath)).To(Equal("1"))
			Expect(accountPath).To(BeADirectory())

			typePath := filepath.Dir(accountPath)
			Expect(filepath.Base(typePath)).To(Equal("right_scripts"))
			Expect(typePath).To(BeADirectory())

			cachePath := filepath.Dir(typePath)
			Expect(cachePath).To(Equal(tempPath))
		})
	})

	Describe("GetMultiCloudImageFile", func() {
		It("gets a cached MultiCloudImage file path", func() {
			path, err := cache.GetMultiCloudImageFile(1, "2345678", 90)
			Expect(err).NotTo(HaveOccurred())
			Expect(filepath.Base(path)).To(Equal("multi_cloud_image.yml"))

			revisionPath := filepath.Dir(path)
			Expect(filepath.Base(revisionPath)).To(Equal("90"))
			Expect(revisionPath).To(BeADirectory())

			idPath := filepath.Dir(revisionPath)
			Expect(filepath.Base(idPath)).To(Equal("2345678"))
			Expect(idPath).To(BeADirectory())

			accountPath := filepath.Dir(idPath)
			Expect(filepath.Base(accountPath)).To(Equal("1"))
			Expect(accountPath).To(BeADirectory())

			typePath := filepath.Dir(accountPath)
			Expect(filepath.Base(typePath)).To(Equal("multi_cloud_images"))
			Expect(typePath).To(BeADirectory())

			cachePath := filepath.Dir(typePath)
			Expect(cachePath).To(Equal(tempPath))
		})
	})
})
