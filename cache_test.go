package main_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	. "github.com/rightscale/right_st"
	"github.com/rightscale/rsc/cm15"
)

var _ = Describe("Cache", func() {
	var (
		tempPath string
		cache    Cache
		utc      *time.Location
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
		t, err := time.Parse("2006/01/02 15:04:05 -0700", "2019/02/03 01:47:25 +0000")
		if err != nil {
			panic(err)
		}
		if t.Format("MST") == "UTC" {
			utc = time.Local
		} else {
			utc = time.FixedZone("", 0)
		}
	})

	AfterEach(func() {
		err := os.RemoveAll(tempPath)
		if err != nil {
			panic(err)
		}
	})

	Describe("GetServerTemplate", func() {
		Context("without a cached ServerTemplate", func() {
			It("returns nil without an error", func() {
				cst, err := cache.GetServerTemplate(1, "2345678", 90)
				Expect(err).NotTo(HaveOccurred())
				Expect(cst).To(BeNil())
			})
		})

		Context("with a cached ServerTemplate", func() {
			var st string

			BeforeEach(func() {
				dir := filepath.Join(tempPath, "server_templates", "1", "2345678", "90")
				if err := os.MkdirAll(dir, 0700); err != nil {
					panic(err)
				}
				st = filepath.Join(dir, "server_template.yaml")
				if err := ioutil.WriteFile(st, []byte(`
Name: Really Cool ServerTemplate
Description: A really cool ServerTemplate
Inputs: {}
RightScripts:
  Boot:
  - Name: Really Cool Script
    Revision: 90
  Operational:
  - Name: Really Cool Script with Attachments
    Revision: 90
MultiCloudImages:
- Name: Ubuntu_18.04_x64
  Revision: 90
`), 0600); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(dir, "item.json"), []byte(`{
  "server_template": {
    "actions": [
      {
        "rel": "commit"
      },
      {
        "rel": "clone"
      },
      {
        "rel": "resolve"
      },
      {
        "rel": "swap_repository"
      },
      {
        "rel": "detect_changes_in_head"
      }
    ],
    "description": "A really cool ServerTemplate",
    "lineage": "https://us-3.rightscale.com/api/acct/1/ec2_server_templates/2345678",
    "links": [
      {
        "href": "/api/server_templates/4567890",
        "rel": "self"
      },
      {
        "href": "/api/server_templates/4567890/multi_cloud_images",
        "rel": "multi_cloud_images"
      },
      {
        "href": "/api/multi_cloud_images/2345678",
        "rel": "default_multi_cloud_image"
      },
      {
        "href": "/api/server_templates/4567890/inputs",
        "rel": "inputs"
      },
      {
        "href": "/api/server_templates/4567890/alert_specs",
        "rel": "alert_specs"
      },
      {
        "href": "/api/server_templates/4567890/runnable_bindings",
        "rel": "runnable_bindings"
      },
      {
        "href": "/api/server_templates/4567890/cookbook_attachments",
        "rel": "cookbook_attachments"
      }
    ],
    "name": "Really Cool ServerTemplate",
    "revision": 90
  },
  "md5": "c656cbfe58c22482a835f1d9ff5d7c47",
  "multi_cloud_images": [
    {
      "actions": [
        {
          "rel": "clone"
        }
      ],
      "description": "Ubuntu_18.04_x64",
      "links": [
        {
          "href": "/api/multi_cloud_images/2345678",
          "rel": "self"
        },
        {
          "href": "/api/multi_cloud_images/2345678/settings",
          "rel": "settings"
        },
        {
          "href": "/api/multi_cloud_images/2345678/matchers",
          "rel": "matchers"
        }
      ],
      "name": "Ubuntu 18.04 x64 LTS Bionic Beaver",
      "revision": 90
	}
  ],
  "right_scripts": [
    {
      "created_at": "2019/02/03 01:47:25 +0000",
      "description": "A really cool script",
      "id": "4567890",
      "lineage": "https://us-3.rightscale.com/api/acct/1/2345678",
      "links": [
        {
          "href": "/api/right_scripts/4567890",
          "rel": "self"
        },
        {
          "href": "/api/right_scripts/4567890/source",
          "rel": "source"
        }
      ],
      "name": "Really Cool Script",
      "revision": 90,
      "updated_at": "2019/02/03 01:47:25 +0000"
    },
    {
      "created_at": "2019/02/03 01:47:25 +0000",
      "description": "A really cool script with attachments",
      "id": "4567890",
      "lineage": "https://us-3.rightscale.com/api/acct/1/2345678",
      "links": [
        {
          "href": "/api/right_scripts/4567890",
          "rel": "self"
        },
        {
          "href": "/api/right_scripts/4567890/source",
          "rel": "source"
        }
      ],
      "name": "Really Cool Script with Attachments",
      "revision": 90,
      "updated_at": "2019/02/03 01:47:25 +0000"
    }
  ],
  "version": 1
}
`), 0600); err != nil {
					panic(err)
				}
			})

			It("returns a cached ServerTemplate", func() {
				cst, err := cache.GetServerTemplate(1, "2345678", 90)
				Expect(err).NotTo(HaveOccurred())
				Expect(cst).To(PointTo(Equal(CachedServerTemplate{
					ServerTemplate: &cm15.ServerTemplate{
						Actions: []map[string]string{
							{"rel": "commit"},
							{"rel": "clone"},
							{"rel": "resolve"},
							{"rel": "swap_repository"},
							{"rel": "detect_changes_in_head"},
						},
						Description: "A really cool ServerTemplate",
						Lineage:     "https://us-3.rightscale.com/api/acct/1/ec2_server_templates/2345678",
						Links: []map[string]string{
							{"href": "/api/server_templates/4567890", "rel": "self"},
							{"href": "/api/server_templates/4567890/multi_cloud_images", "rel": "multi_cloud_images"},
							{"href": "/api/multi_cloud_images/2345678", "rel": "default_multi_cloud_image"},
							{"href": "/api/server_templates/4567890/inputs", "rel": "inputs"},
							{"href": "/api/server_templates/4567890/alert_specs", "rel": "alert_specs"},
							{"href": "/api/server_templates/4567890/runnable_bindings", "rel": "runnable_bindings"},
							{"href": "/api/server_templates/4567890/cookbook_attachments", "rel": "cookbook_attachments"},
						},
						Name:     "Really Cool ServerTemplate",
						Revision: 90,
					},
					File:   st,
					MD5Sum: "c656cbfe58c22482a835f1d9ff5d7c47",
					MultiCloudImages: []*cm15.MultiCloudImage{
						{
							Actions:     []map[string]string{{"rel": "clone"}},
							Description: "Ubuntu_18.04_x64",
							Links: []map[string]string{
								{
									"href": "/api/multi_cloud_images/2345678",
									"rel":  "self",
								},
								{
									"rel":  "settings",
									"href": "/api/multi_cloud_images/2345678/settings",
								},
								{
									"href": "/api/multi_cloud_images/2345678/matchers",
									"rel":  "matchers",
								},
							},
							Name:     "Ubuntu 18.04 x64 LTS Bionic Beaver",
							Revision: 90,
						},
					},
					RightScripts: []*cm15.RightScript{
						{
							CreatedAt:   &cm15.RubyTime{time.Date(2019, 2, 3, 1, 47, 25, 0, utc)},
							Description: "A really cool script",
							Id:          "4567890",
							Lineage:     "https://us-3.rightscale.com/api/acct/1/2345678",
							Links: []map[string]string{
								{"href": "/api/right_scripts/4567890", "rel": "self"},
								{"href": "/api/right_scripts/4567890/source", "rel": "source"},
							},
							Name:      "Really Cool Script",
							Revision:  90,
							UpdatedAt: &cm15.RubyTime{time.Date(2019, 2, 3, 1, 47, 25, 0, utc)},
						},
						{
							CreatedAt:   &cm15.RubyTime{time.Date(2019, 2, 3, 1, 47, 25, 0, utc)},
							Description: "A really cool script with attachments",
							Id:          "4567890",
							Lineage:     "https://us-3.rightscale.com/api/acct/1/2345678",
							Links: []map[string]string{
								{"href": "/api/right_scripts/4567890", "rel": "self"},
								{"href": "/api/right_scripts/4567890/source", "rel": "source"},
							},
							Name:      "Really Cool Script with Attachments",
							Revision:  90,
							UpdatedAt: &cm15.RubyTime{time.Date(2019, 2, 3, 1, 47, 25, 0, utc)},
						},
					},
					Version: CacheVersion,
				})))
			})
		})

		Context("with a different version cached ServerTemplate", func() {
			var st string

			BeforeEach(func() {
				dir := filepath.Join(tempPath, "server_templates", "1", "2345678", "90")
				if err := os.MkdirAll(dir, 0700); err != nil {
					panic(err)
				}
				st = filepath.Join(dir, "server_template.yaml")
				if err := ioutil.WriteFile(st, []byte(`
Name: Really Cool ServerTemplate
Description: A really cool ServerTemplate
Inputs: {}
RightScripts:
  Boot:
  - Name: Really Cool Script
    Revision: 90
  Operational:
  - Name: Really Cool Script with Attachments
    Revision: 90
MultiCloudImages:
- Name: Ubuntu_18.04_x64
  Revision: 90
`), 0600); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(dir, "item.json"), []byte(`{
  "server_template": {
    "actions": [
      {
        "rel": "commit"
      },
      {
        "rel": "clone"
      },
      {
        "rel": "resolve"
      },
      {
        "rel": "swap_repository"
      },
      {
        "rel": "detect_changes_in_head"
      }
    ],
    "description": "A really cool ServerTemplate",
    "lineage": "https://us-3.rightscale.com/api/acct/1/ec2_server_templates/2345678",
    "links": [
      {
        "href": "/api/server_templates/4567890",
        "rel": "self"
      },
      {
        "href": "/api/server_templates/4567890/multi_cloud_images",
        "rel": "multi_cloud_images"
      },
      {
        "href": "/api/multi_cloud_images/2345678",
        "rel": "default_multi_cloud_image"
      },
      {
        "href": "/api/server_templates/4567890/inputs",
        "rel": "inputs"
      },
      {
        "href": "/api/server_templates/4567890/alert_specs",
        "rel": "alert_specs"
      },
      {
        "href": "/api/server_templates/4567890/runnable_bindings",
        "rel": "runnable_bindings"
      },
      {
        "href": "/api/server_templates/4567890/cookbook_attachments",
        "rel": "cookbook_attachments"
      }
    ],
    "name": "Really Cool ServerTemplate",
    "revision": 90
  },
  "md5": "c656cbfe58c22482a835f1d9ff5d7c47",
  "version": 0
}
`), 0600); err != nil {
					panic(err)
				}
			})

			It("returns nil without an error and removes the cached ServerTemplate directory", func() {
				cst, err := cache.GetServerTemplate(1, "2345678", 90)
				Expect(err).NotTo(HaveOccurred())
				Expect(cst).To(BeNil())

				Expect(st).NotTo(BeAnExistingFile())
				Expect(filepath.Dir(st)).NotTo(BeAnExistingFile())
			})
		})

		Context("with an invalid cached ServerTemplate", func() {
			var st string

			BeforeEach(func() {
				dir := filepath.Join(tempPath, "server_templates", "1", "2345678", "90")
				if err := os.MkdirAll(dir, 0700); err != nil {
					panic(err)
				}
				st = filepath.Join(dir, "server_template.yaml")
				if err := ioutil.WriteFile(st, []byte(`
Name: Really Cool ServerTemplate
Description: A really cool ServerTemplate
Inputs: {}
RightScripts:
  Boot:
  - Name: Really Cool Script
    Revision: 90
  Decommission:
  - Name: Really Cool Script with Attachments
    Revision: 90
MultiCloudImages:
- Name: Ubuntu_18.04_x64
  Revision: 90
`), 0600); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(dir, "item.json"), []byte(`{
  "server_template": {
    "actions": [
      {
        "rel": "commit"
      },
      {
        "rel": "clone"
      },
      {
        "rel": "resolve"
      },
      {
        "rel": "swap_repository"
      },
      {
        "rel": "detect_changes_in_head"
      }
    ],
    "description": "A really cool ServerTemplate",
    "lineage": "https://us-3.rightscale.com/api/acct/1/ec2_server_templates/2345678",
    "links": [
      {
        "href": "/api/server_templates/4567890",
        "rel": "self"
      },
      {
        "href": "/api/server_templates/4567890/multi_cloud_images",
        "rel": "multi_cloud_images"
      },
      {
        "href": "/api/multi_cloud_images/2345678",
        "rel": "default_multi_cloud_image"
      },
      {
        "href": "/api/server_templates/4567890/inputs",
        "rel": "inputs"
      },
      {
        "href": "/api/server_templates/4567890/alert_specs",
        "rel": "alert_specs"
      },
      {
        "href": "/api/server_templates/4567890/runnable_bindings",
        "rel": "runnable_bindings"
      },
      {
        "href": "/api/server_templates/4567890/cookbook_attachments",
        "rel": "cookbook_attachments"
      }
    ],
    "name": "Really Cool ServerTemplate",
    "revision": 90
  },
  "multi_cloud_images": [
    {
	  "actions": [
	    {
		  "rel": "clone"
	    }
	  ],
	  "description": "Ubuntu_18.04_x64",
	  "links": [
	    {
		  "href": "/api/multi_cloud_images/2345678",
		  "rel": "self"
	    },
	    {
		  "href": "/api/multi_cloud_images/2345678/settings",
		  "rel": "settings"
	    },
	    {
		  "href": "/api/multi_cloud_images/2345678/matchers",
		  "rel": "matchers"
	    }
	  ],
	  "name": "Ubuntu 18.04 x64 LTS Bionic Beaver",
	  "revision": 90
    }
  ],
  "right_scripts": [
    {
      "created_at": "2019/02/03 01:47:25 +0000",
      "description": "A really cool script",
      "id": "4567890",
      "lineage": "https://us-3.rightscale.com/api/acct/1/2345678",
      "links": [
        {
          "href": "/api/right_scripts/4567890",
          "rel": "self"
        },
        {
          "href": "/api/right_scripts/4567890/source",
          "rel": "source"
        }
      ],
      "name": "Really Cool Script",
      "revision": 90,
      "updated_at": "2019/02/03 01:47:25 +0000"
    },
    {
      "created_at": "2019/02/03 01:47:25 +0000",
      "description": "A really cool script with attachments",
      "id": "4567890",
      "lineage": "https://us-3.rightscale.com/api/acct/1/2345678",
      "links": [
        {
          "href": "/api/right_scripts/4567890",
          "rel": "self"
        },
        {
          "href": "/api/right_scripts/4567890/source",
          "rel": "source"
        }
      ],
      "name": "Really Cool Script with Attachments",
      "revision": 90,
      "updated_at": "2019/02/03 01:47:25 +0000"
    }
  ],
  "md5": "c656cbfe58c22482a835f1d9ff5d7c47"
}
`), 0600); err != nil {
					panic(err)
				}
			})

			It("returns nil without an error and removes the cached ServerTemplate directory", func() {
				cst, err := cache.GetServerTemplate(1, "2345678", 90)
				Expect(err).NotTo(HaveOccurred())
				Expect(cst).To(BeNil())

				Expect(st).NotTo(BeAnExistingFile())
				Expect(filepath.Dir(st)).NotTo(BeAnExistingFile())
			})
		})
	})

	Describe("GetServerTemplateFile", func() {
		It("gets a cached ServerTemplate file path", func() {
			path, err := cache.GetServerTemplateFile(1, "2345678", 90)
			Expect(err).NotTo(HaveOccurred())
			Expect(filepath.Base(path)).To(Equal("server_template.yaml"))

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

	Describe("PutServerTemplate", func() {
		BeforeEach(func() {
			dir := filepath.Join(tempPath, "server_templates", "1", "2345678", "90")
			if err := os.MkdirAll(dir, 0700); err != nil {
				panic(err)
			}
			if err := ioutil.WriteFile(filepath.Join(dir, "server_template.yaml"), []byte(`
Name: Really Cool ServerTemplate
Description: A really cool ServerTemplate
Inputs: {}
RightScripts:
  Boot:
  - Name: Really Cool Script
    Revision: 90
  Operational:
  - Name: Really Cool Script with Attachments
    Revision: 90
MultiCloudImages:
- Name: Ubuntu_18.04_x64
  Revision: 90
`), 0600); err != nil {
				panic(err)
			}
		})

		It("writes an item JSON file", func() {
			st := cm15.ServerTemplate{
				Actions: []map[string]string{
					{"rel": "commit"},
					{"rel": "clone"},
					{"rel": "resolve"},
					{"rel": "swap_repository"},
					{"rel": "detect_changes_in_head"},
				},
				Description: "A really cool ServerTemplate",
				Lineage:     "https://us-3.rightscale.com/api/acct/1/ec2_server_templates/2345678",
				Links: []map[string]string{
					{
						"href": "/api/server_templates/4567890",
						"rel":  "self",
					},
					{
						"href": "/api/server_templates/4567890/multi_cloud_images",
						"rel":  "multi_cloud_images",
					},
					{
						"href": "/api/multi_cloud_images/2345678",
						"rel":  "default_multi_cloud_image",
					},
					{
						"href": "/api/server_templates/4567890/inputs",
						"rel":  "inputs",
					},
					{
						"href": "/api/server_templates/4567890/alert_specs",
						"rel":  "alert_specs",
					},
					{
						"href": "/api/server_templates/4567890/runnable_bindings",
						"rel":  "runnable_bindings",
					},
					{
						"href": "/api/server_templates/4567890/cookbook_attachments",
						"rel":  "cookbook_attachments",
					},
				},
				Name:     "Really Cool ServerTemplate",
				Revision: 90,
			}
			mcis := []*cm15.MultiCloudImage{
				{
					Actions:     []map[string]string{{"rel": "clone"}},
					Description: "Ubuntu_18.04_x64",
					Links: []map[string]string{
						{"href": "/api/multi_cloud_images/2345678", "rel": "self"},
						{"href": "/api/multi_cloud_images/2345678/settings", "rel": "settings"},
						{"href": "/api/multi_cloud_images/2345678/matchers", "rel": "matchers"},
					},
					Name:     "Ubuntu 18.04 x64 LTS Bionic Beaver",
					Revision: 90,
				},
			}
			scripts := []*cm15.RightScript{
				{
					CreatedAt:   &cm15.RubyTime{time.Date(2019, 2, 3, 1, 47, 25, 0, utc)},
					Description: "A really cool script",
					Id:          "4567890",
					Lineage:     "https://us-3.rightscale.com/api/acct/1/2345678",
					Links: []map[string]string{
						{"href": "/api/right_scripts/4567890", "rel": "self"},
						{"href": "/api/right_scripts/4567890/source", "rel": "source"},
					},
					Name:      "Really Cool Script",
					Revision:  90,
					UpdatedAt: &cm15.RubyTime{time.Date(2019, 2, 3, 1, 47, 25, 0, utc)},
				},
				{
					CreatedAt:   &cm15.RubyTime{time.Date(2019, 2, 3, 1, 47, 25, 0, utc)},
					Description: "A really cool script with attachments",
					Id:          "4567890",
					Lineage:     "https://us-3.rightscale.com/api/acct/1/2345678",
					Links: []map[string]string{
						{"href": "/api/right_scripts/4567890", "rel": "self"},
						{"href": "/api/right_scripts/4567890/source", "rel": "source"},
					},
					Name:      "Really Cool Script with Attachments",
					Revision:  90,
					UpdatedAt: &cm15.RubyTime{time.Date(2019, 2, 3, 1, 47, 25, 0, utc)},
				},
			}
			err := cache.PutServerTemplate(1, "2345678", 90, &st, mcis, scripts)
			Expect(err).NotTo(HaveOccurred())

			item := filepath.Join(tempPath, "server_templates", "1", "2345678", "90", "item.json")
			Expect(item).To(BeARegularFile())
			Expect(ioutil.ReadFile(item)).To(MatchJSON(`{
  "server_template": {
    "actions": [
      {
        "rel": "commit"
      },
      {
        "rel": "clone"
      },
      {
        "rel": "resolve"
      },
      {
        "rel": "swap_repository"
      },
      {
        "rel": "detect_changes_in_head"
      }
    ],
    "description": "A really cool ServerTemplate",
    "lineage": "https://us-3.rightscale.com/api/acct/1/ec2_server_templates/2345678",
    "links": [
      {
        "href": "/api/server_templates/4567890",
        "rel": "self"
      },
      {
        "href": "/api/server_templates/4567890/multi_cloud_images",
        "rel": "multi_cloud_images"
      },
      {
        "href": "/api/multi_cloud_images/2345678",
        "rel": "default_multi_cloud_image"
      },
      {
        "href": "/api/server_templates/4567890/inputs",
        "rel": "inputs"
      },
      {
        "href": "/api/server_templates/4567890/alert_specs",
        "rel": "alert_specs"
      },
      {
        "href": "/api/server_templates/4567890/runnable_bindings",
        "rel": "runnable_bindings"
      },
      {
        "href": "/api/server_templates/4567890/cookbook_attachments",
        "rel": "cookbook_attachments"
      }
    ],
    "name": "Really Cool ServerTemplate",
    "revision": 90
  },
  "md5": "c656cbfe58c22482a835f1d9ff5d7c47",
  "multi_cloud_images": [
    {
      "actions": [
        {
          "rel": "clone"
        }
      ],
      "description": "Ubuntu_18.04_x64",
      "links": [
        {
          "href": "/api/multi_cloud_images/2345678",
          "rel": "self"
        },
        {
          "href": "/api/multi_cloud_images/2345678/settings",
          "rel": "settings"
        },
        {
          "href": "/api/multi_cloud_images/2345678/matchers",
          "rel": "matchers"
        }
      ],
      "name": "Ubuntu 18.04 x64 LTS Bionic Beaver",
      "revision": 90
	}
  ],
  "right_scripts": [
    {
      "created_at": "2019/02/03 01:47:25 +0000",
      "description": "A really cool script",
      "id": "4567890",
      "lineage": "https://us-3.rightscale.com/api/acct/1/2345678",
      "links": [
        {
          "href": "/api/right_scripts/4567890",
          "rel": "self"
        },
        {
          "href": "/api/right_scripts/4567890/source",
          "rel": "source"
        }
      ],
      "name": "Really Cool Script",
      "revision": 90,
      "updated_at": "2019/02/03 01:47:25 +0000"
    },
    {
      "created_at": "2019/02/03 01:47:25 +0000",
      "description": "A really cool script with attachments",
      "id": "4567890",
      "lineage": "https://us-3.rightscale.com/api/acct/1/2345678",
      "links": [
        {
          "href": "/api/right_scripts/4567890",
          "rel": "self"
        },
        {
          "href": "/api/right_scripts/4567890/source",
          "rel": "source"
        }
      ],
      "name": "Really Cool Script with Attachments",
      "revision": 90,
      "updated_at": "2019/02/03 01:47:25 +0000"
    }
  ],
  "version": 1
}
`))
		})
	})

	Describe("GetRightScript", func() {
		Context("without a cached RightScript", func() {
			It("returns nil without an error", func() {
				crs, err := cache.GetRightScript(1, "2345678", 90)
				Expect(err).NotTo(HaveOccurred())
				Expect(crs).To(BeNil())
			})
		})

		Context("with a cached RightScript", func() {
			var rs string

			BeforeEach(func() {
				dir := filepath.Join(tempPath, "right_scripts", "1", "2345678", "90")
				if err := os.MkdirAll(dir, 0700); err != nil {
					panic(err)
				}
				rs = filepath.Join(dir, "right_script")
				if err := ioutil.WriteFile(rs, []byte(`#!/bin/bash
# ---
# RightScript Name: Really Cool Script
# Description: A really cool script
# Inputs: {}
# Attachments: []
# ...

echo 'Really cool script!'
`), 0600); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(dir, "item.json"), []byte(`{
  "right_script": {
    "created_at": "2019/02/03 01:47:25 +0000",
    "description": "A really cool script",
    "id": "4567890",
    "lineage": "https://us-3.rightscale.com/api/acct/1/2345678",
    "links": [
      {
        "href": "/api/right_scripts/4567890",
        "rel": "self"
      },
      {
        "href": "/api/right_scripts/4567890/source",
        "rel": "source"
      }
    ],
    "name": "Really Cool Script",
    "revision": 90,
    "updated_at": "2019/02/03 01:47:25 +0000"
  },
  "md5": "130cd1afe75c80631f89c69bfb3052ab",
  "version": 1
}
`), 0600); err != nil {
					panic(err)
				}
			})

			It("returns a cached RightScript", func() {
				crs, err := cache.GetRightScript(1, "2345678", 90)
				Expect(err).NotTo(HaveOccurred())
				Expect(crs).NotTo(BeNil())
				Expect(crs).To(PointTo(Equal(CachedRightScript{
					&cm15.RightScript{
						CreatedAt:   &cm15.RubyTime{time.Date(2019, 2, 3, 1, 47, 25, 0, utc).In(crs.RightScript.CreatedAt.Location())},
						Description: "A really cool script",
						Id:          "4567890",
						Lineage:     "https://us-3.rightscale.com/api/acct/1/2345678",
						Links: []map[string]string{
							{
								"href": "/api/right_scripts/4567890",
								"rel":  "self",
							},
							{
								"href": "/api/right_scripts/4567890/source",
								"rel":  "source",
							},
						},
						Name:      "Really Cool Script",
						Revision:  90,
						UpdatedAt: &cm15.RubyTime{time.Date(2019, 2, 3, 1, 47, 25, 0, utc).In(crs.RightScript.UpdatedAt.Location())},
					},
					rs, "130cd1afe75c80631f89c69bfb3052ab", nil, CacheVersion,
				})))
			})
		})

		Context("with a cached RightScript with attachments", func() {
			var rs string

			BeforeEach(func() {
				dir := filepath.Join(tempPath, "right_scripts", "1", "2345678", "90")
				err := os.MkdirAll(dir, 0700)
				if err != nil {
					panic(err)
				}
				rs = filepath.Join(dir, "right_script")
				if err := ioutil.WriteFile(rs, []byte(`#!/bin/bash
# ---
# RightScript Name: Really Cool Script with Attachments
# Description: A really cool script with attachments
# Inputs: {}
# Attachments:
# - a.txt
# - b.txt

cat "$RS_ATTACH_DIR"/*.txt
`), 0600); err != nil {
					panic(err)
				}
				attachments := filepath.Join(dir, "attachments")
				if err := os.Mkdir(attachments, 0700); err != nil {
					panic(err)
				}
				if err = ioutil.WriteFile(filepath.Join(attachments, "a.txt"), []byte("A really cool text file!\n"), 0600); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(attachments, "b.txt"), []byte("Another really cool text file!\n"), 0600); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(dir, "item.json"), []byte(`{
  "right_script": {
    "created_at": "2019/02/03 01:47:25 +0000",
    "description": "A really cool script with attachments",
    "id": "4567890",
    "lineage": "https://us-3.rightscale.com/api/acct/1/2345678",
    "links": [
      {
        "href": "/api/right_scripts/4567890",
        "rel": "self"
      },
      {
        "href": "/api/right_scripts/4567890/source",
        "rel": "source"
      }
    ],
    "name": "Really Cool Script with Attachments",
    "revision": 90,
    "updated_at": "2019/02/03 01:47:25 +0000"
  },
  "md5": "9dabfc14b729875fa9dd499bc9c79f95",
  "attachments": {
    "a.txt": "5e9a07cc7a7353a067f0060f5eb968ed",
    "b.txt": "f67753368d236a5819ec172b2a18d841"
  },
  "version": 1
}
`), 0600); err != nil {
					panic(err)
				}
			})

			It("returns a cached RightScript", func() {
				crs, err := cache.GetRightScript(1, "2345678", 90)
				Expect(err).NotTo(HaveOccurred())
				Expect(crs).NotTo(BeNil())
				Expect(crs).To(PointTo(Equal(CachedRightScript{
					&cm15.RightScript{
						CreatedAt:   &cm15.RubyTime{time.Date(2019, 2, 3, 1, 47, 25, 0, utc).In(crs.RightScript.CreatedAt.Location())},
						Description: "A really cool script with attachments",
						Id:          "4567890",
						Lineage:     "https://us-3.rightscale.com/api/acct/1/2345678",
						Links: []map[string]string{
							{
								"href": "/api/right_scripts/4567890",
								"rel":  "self",
							},
							{
								"href": "/api/right_scripts/4567890/source",
								"rel":  "source",
							},
						},
						Name:      "Really Cool Script with Attachments",
						Revision:  90,
						UpdatedAt: &cm15.RubyTime{time.Date(2019, 2, 3, 1, 47, 25, 0, utc).In(crs.RightScript.UpdatedAt.Location())},
					},
					rs, "9dabfc14b729875fa9dd499bc9c79f95",
					map[string]string{
						"a.txt": "5e9a07cc7a7353a067f0060f5eb968ed",
						"b.txt": "f67753368d236a5819ec172b2a18d841",
					},
					CacheVersion,
				})))
			})
		})

		Context("with a different version cached RightScript", func() {
			var rs string

			BeforeEach(func() {
				dir := filepath.Join(tempPath, "right_scripts", "1", "2345678", "90")
				if err := os.MkdirAll(dir, 0700); err != nil {
					panic(err)
				}
				rs = filepath.Join(dir, "right_script")
				if err := ioutil.WriteFile(rs, []byte(`#!/bin/bash
# ---
# RightScript Name: Really Cool Script
# Description: A really cool script
# Inputs: {}
# Attachments: []
# ...

echo 'Really cool script!'
`), 0600); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(dir, "item.json"), []byte(`{
  "right_script": {
    "created_at": "2019/02/03 01:47:25 +0000",
    "description": "A really cool script",
    "id": "4567890",
    "lineage": "https://us-3.rightscale.com/api/acct/1/2345678",
    "links": [
      {
        "href": "/api/right_scripts/4567890",
        "rel": "self"
      },
      {
        "href": "/api/right_scripts/4567890/source",
        "rel": "source"
      }
    ],
    "name": "Really Cool Script",
    "revision": 90,
    "updated_at": "2019/02/03 01:47:25 +0000"
  },
  "md5": "130cd1afe75c80631f89c69bfb3052ab",
  "version": 0
}
`), 0600); err != nil {
					panic(err)
				}
			})

			It("returns nil without an error and removes the cached RightScript directory", func() {
				crs, err := cache.GetRightScript(1, "2345678", 90)
				Expect(err).NotTo(HaveOccurred())
				Expect(crs).To(BeNil())

				Expect(rs).NotTo(BeAnExistingFile())
				Expect(filepath.Dir(rs)).NotTo(BeAnExistingFile())
			})
		})

		Context("with an invalid cached RightScript", func() {
			var rs string

			BeforeEach(func() {
				dir := filepath.Join(tempPath, "right_scripts", "1", "2345678", "90")
				if err := os.MkdirAll(dir, 0700); err != nil {
					panic(err)
				}
				rs = filepath.Join(dir, "right_script")
				if err := ioutil.WriteFile(rs, []byte(`#!/bin/bash
# ---
# RightScript Name: Really Bad Script
# Description: A really bad script
# Inputs: {}
# Attachments: []
# ...

echo 'Really bad script!'
`), 0600); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(dir, "item.json"), []byte(`{
  "right_script": {
    "created_at": "2019/02/03 01:47:25 +0000",
    "description": "A really cool script",
    "id": "4567890",
    "lineage": "https://us-3.rightscale.com/api/acct/1/2345678",
    "links": [
      {
        "href": "/api/right_scripts/4567890",
        "rel": "self"
      },
      {
        "href": "/api/right_scripts/4567890/source",
        "rel": "source"
      }
    ],
    "name": "Really Cool Script",
    "revision": 90,
    "updated_at": "2019/02/03 01:47:25 +0000"
  },
  "md5": "130cd1afe75c80631f89c69bfb3052ab"
}
`), 0600); err != nil {
					panic(err)
				}
			})

			It("returns nil without an error and removes the cached RightScript directory", func() {
				crs, err := cache.GetRightScript(1, "2345678", 90)
				Expect(err).NotTo(HaveOccurred())
				Expect(crs).To(BeNil())

				Expect(rs).NotTo(BeAnExistingFile())
				Expect(filepath.Dir(rs)).NotTo(BeAnExistingFile())
			})
		})

		Context("with a cached RightScript with invalid attachments", func() {
			var rs string

			BeforeEach(func() {
				dir := filepath.Join(tempPath, "right_scripts", "1", "2345678", "90")
				if err := os.MkdirAll(dir, 0700); err != nil {
					panic(err)
				}
				rs = filepath.Join(dir, "right_script")
				if err := ioutil.WriteFile(rs, []byte(`#!/bin/bash
# ---
# RightScript Name: Really Cool Script with Attachments
# Description: A really cool script with attachments
# Inputs: {}
# Attachments:
# - a.txt
# - b.txt

cat "$RS_ATTACH_DIR"/*.txt
`), 0600); err != nil {
					panic(err)
				}
				attachments := filepath.Join(dir, "attachments")
				if err := os.Mkdir(attachments, 0700); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(attachments, "a.txt"), []byte("A really cool text file!\n"), 0600); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(attachments, "b.txt"), []byte("A really bad text file!\n"), 0600); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(dir, "item.json"), []byte(`{
  "right_script": {
    "created_at": "2019/02/03 01:47:25 +0000",
    "description": "A really cool script with attachments",
    "id": "4567890",
    "lineage": "https://us-3.rightscale.com/api/acct/1/2345678",
    "links": [
      {
        "href": "/api/right_scripts/4567890",
        "rel": "self"
      },
      {
        "href": "/api/right_scripts/4567890/source",
        "rel": "source"
      }
    ],
    "name": "Really Cool Script with Attachments",
    "revision": 90,
    "updated_at": "2019/02/03 01:47:25 +0000"
  },
  "md5": "9dabfc14b729875fa9dd499bc9c79f95",
  "attachments": {
    "a.txt": "5e9a07cc7a7353a067f0060f5eb968ed",
    "b.txt": "f67753368d236a5819ec172b2a18d841"
  }
}
`), 0600); err != nil {
					panic(err)
				}
			})

			It("returns nil without an error and removes the cached RightScript directory", func() {
				crs, err := cache.GetRightScript(1, "2345678", 90)
				Expect(err).NotTo(HaveOccurred())
				Expect(crs).To(BeNil())

				Expect(rs).NotTo(BeAnExistingFile())
				Expect(filepath.Dir(rs)).NotTo(BeAnExistingFile())
			})
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

	Describe("PutRightScript", func() {
		Context("without attachments", func() {
			BeforeEach(func() {
				dir := filepath.Join(tempPath, "right_scripts", "1", "2345678", "90")
				err := os.MkdirAll(dir, 0700)
				if err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(dir, "right_script"), []byte(`#!/bin/bash
# ---
# RightScript Name: Really Cool Script
# Description: A really cool script
# Inputs: {}
# Attachments: []
# ...

echo 'Really cool script!'
`), 0600); err != nil {
					panic(err)
				}
			})

			It("writes an item JSON file", func() {
				rs := cm15.RightScript{
					CreatedAt:   &cm15.RubyTime{time.Date(2019, 2, 3, 1, 47, 25, 0, utc)},
					Description: "A really cool script",
					Id:          "4567890",
					Lineage:     "https://us-3.rightscale.com/api/acct/1/2345678",
					Links: []map[string]string{
						{
							"href": "/api/right_scripts/4567890",
							"rel":  "self",
						},
						{
							"href": "/api/right_scripts/4567890/source",
							"rel":  "source",
						},
					},
					Name:      "Really Cool Script",
					Revision:  90,
					UpdatedAt: &cm15.RubyTime{time.Date(2019, 2, 3, 1, 47, 25, 0, utc)},
				}
				err := cache.PutRightScript(1, "2345678", 90, &rs, nil)
				Expect(err).NotTo(HaveOccurred())

				item := filepath.Join(tempPath, "right_scripts", "1", "2345678", "90", "item.json")
				Expect(item).To(BeARegularFile())
				Expect(ioutil.ReadFile(item)).To(MatchJSON(`{
  "right_script": {
    "created_at": "2019/02/03 01:47:25 +0000",
    "description": "A really cool script",
    "id": "4567890",
    "lineage": "https://us-3.rightscale.com/api/acct/1/2345678",
    "links": [
      {
        "href": "/api/right_scripts/4567890",
        "rel": "self"
      },
      {
        "href": "/api/right_scripts/4567890/source",
        "rel": "source"
      }
    ],
    "name": "Really Cool Script",
    "revision": 90,
    "updated_at": "2019/02/03 01:47:25 +0000"
  },
  "md5": "130cd1afe75c80631f89c69bfb3052ab",
  "version": 1
}`))
			})
		})

		Context("with attachments", func() {
			BeforeEach(func() {
				dir := filepath.Join(tempPath, "right_scripts", "1", "2345678", "90")
				if err := os.MkdirAll(dir, 0700); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(dir, "right_script"), []byte(`#!/bin/bash
# ---
# RightScript Name: Really Cool Script with Attachments
# Description: A really cool script with attachments
# Inputs: {}
# Attachments:
# - a.txt
# - b.txt

cat "$RS_ATTACH_DIR"/*.txt
`), 0600); err != nil {
					panic(err)
				}
				attachments := filepath.Join(dir, "attachments")
				if err := os.Mkdir(attachments, 0700); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(attachments, "a.txt"), []byte("A really cool text file!\n"), 0600); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(attachments, "b.txt"), []byte("Another really cool text file!\n"), 0600); err != nil {
					panic(err)
				}
			})

			It("writes an item JSON file", func() {
				rs := cm15.RightScript{
					CreatedAt:   &cm15.RubyTime{time.Date(2019, 2, 3, 1, 47, 25, 0, utc)},
					Description: "A really cool script with attachments",
					Id:          "4567890",
					Lineage:     "https://us-3.rightscale.com/api/acct/1/2345678",
					Links: []map[string]string{
						{
							"href": "/api/right_scripts/4567890",
							"rel":  "self",
						},
						{
							"href": "/api/right_scripts/4567890/source",
							"rel":  "source",
						},
					},
					Name:      "Really Cool Script with Attachments",
					Revision:  90,
					UpdatedAt: &cm15.RubyTime{time.Date(2019, 2, 3, 1, 47, 25, 0, utc)},
				}
				err := cache.PutRightScript(1, "2345678", 90, &rs, []string{"a.txt", "b.txt"})
				Expect(err).NotTo(HaveOccurred())

				item := filepath.Join(tempPath, "right_scripts", "1", "2345678", "90", "item.json")
				Expect(item).To(BeARegularFile())
				Expect(ioutil.ReadFile(item)).To(MatchJSON(`{
  "right_script": {
    "created_at": "2019/02/03 01:47:25 +0000",
    "description": "A really cool script with attachments",
    "id": "4567890",
    "lineage": "https://us-3.rightscale.com/api/acct/1/2345678",
    "links": [
      {
        "href": "/api/right_scripts/4567890",
        "rel": "self"
      },
      {
        "href": "/api/right_scripts/4567890/source",
        "rel": "source"
      }
    ],
    "name": "Really Cool Script with Attachments",
    "revision": 90,
    "updated_at": "2019/02/03 01:47:25 +0000"
  },
  "md5": "9dabfc14b729875fa9dd499bc9c79f95",
  "attachments": {
    "a.txt": "5e9a07cc7a7353a067f0060f5eb968ed",
    "b.txt": "f67753368d236a5819ec172b2a18d841"
  },
  "version": 1
}
`))
			})
		})
	})

	Describe("GetMultiCloudImage", func() {
		Context("without a cached MultiCloudImage", func() {
			It("returns nil without an error", func() {
				cmci, err := cache.GetMultiCloudImage(1, "2345678", 90)
				Expect(err).NotTo(HaveOccurred())
				Expect(cmci).To(BeNil())
			})
		})

		Context("with a cached MultiCloudImage", func() {
			var mci string

			BeforeEach(func() {
				dir := filepath.Join(tempPath, "multi_cloud_images", "1", "2345678", "90")
				if err := os.MkdirAll(dir, 0700); err != nil {
					panic(err)
				}
				mci = filepath.Join(dir, "multi_cloud_image.yaml")
				if err := ioutil.WriteFile(mci, []byte(`Name: Ubuntu_18.04_x64
Description: Ubuntu 18.04 x64 LTS Bionic Beaver
Tags:
- rs_agent:mime_shellscript=https://rightlink.rightscale.com/rll/10.6.0/rightlink.boot.sh
- rs_agent:type=right_link_lite
Settings:
- Cloud: EC2 us-east-1
  Instance Type: c5.large
  Image: ami-1234567890abcdefg
`), 0600); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(dir, "item.json"), []byte(`{
  "multi_cloud_image": {
    "actions": [
      {
        "rel": "clone"
      }
    ],
    "description": "Ubuntu_18.04_x64",
    "links": [
      {
        "href": "/api/multi_cloud_images/2345678",
        "rel": "self"
      },
      {
        "href": "/api/multi_cloud_images/2345678/settings",
        "rel": "settings"
      },
      {
        "href": "/api/multi_cloud_images/2345678/matchers",
        "rel": "matchers"
      }
    ],
    "name": "Ubuntu 18.04 x64 LTS Bionic Beaver",
    "revision": 90
  },
  "md5": "64767c6b5a0c1b446c0cc27eb40cb832",
  "version": 1
}
`), 0600); err != nil {
					panic(err)
				}
			})

			It("returns a cached MultiCloudImage", func() {
				cmci, err := cache.GetMultiCloudImage(1, "2345678", 90)
				Expect(err).NotTo(HaveOccurred())
				Expect(cmci).To(PointTo(Equal(CachedMultiCloudImage{
					&cm15.MultiCloudImage{
						Actions:     []map[string]string{{"rel": "clone"}},
						Description: "Ubuntu_18.04_x64",
						Links: []map[string]string{
							{
								"href": "/api/multi_cloud_images/2345678",
								"rel":  "self",
							},
							{
								"href": "/api/multi_cloud_images/2345678/settings",
								"rel":  "settings",
							},
							{
								"href": "/api/multi_cloud_images/2345678/matchers",
								"rel":  "matchers",
							},
						},
						Name:     "Ubuntu 18.04 x64 LTS Bionic Beaver",
						Revision: 90,
					},
					mci, "64767c6b5a0c1b446c0cc27eb40cb832", CacheVersion,
				})))
			})
		})

		Context("with a different version cached MultiCloudImage", func() {
			var mci string

			BeforeEach(func() {
				dir := filepath.Join(tempPath, "multi_cloud_images", "1", "2345678", "90")
				if err := os.MkdirAll(dir, 0700); err != nil {
					panic(err)
				}
				mci = filepath.Join(dir, "multi_cloud_image.yaml")
				if err := ioutil.WriteFile(mci, []byte(`Name: Ubuntu_18.04_x64
Description: Ubuntu 18.04 x64 LTS Bionic Beaver
Tags:
- rs_agent:mime_shellscript=https://rightlink.rightscale.com/rll/10.6.0/rightlink.boot.sh
- rs_agent:type=right_link_lite
Settings:
- Cloud: EC2 us-east-1
  Instance Type: c5.large
  Image: ami-1234567890abcdefg
`), 0600); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(dir, "item.json"), []byte(`{
  "multi_cloud_image": {
    "actions": [
      {
        "rel": "clone"
      }
    ],
    "description": "Ubuntu_18.04_x64",
    "links": [
      {
        "href": "/api/multi_cloud_images/2345678",
        "rel": "self"
      },
      {
        "href": "/api/multi_cloud_images/2345678/settings",
        "rel": "settings"
      },
      {
        "href": "/api/multi_cloud_images/2345678/matchers",
        "rel": "matchers"
      }
    ],
    "name": "Ubuntu 18.04 x64 LTS Bionic Beaver",
    "revision": 90
  },
  "md5": "64767c6b5a0c1b446c0cc27eb40cb832",
  "version": 0
}
`), 0600); err != nil {
					panic(err)
				}
			})

			It("returns nil without an error and removes the cached MultiCloudImage directory", func() {
				cmci, err := cache.GetMultiCloudImage(1, "2345678", 90)
				Expect(err).NotTo(HaveOccurred())
				Expect(cmci).To(BeNil())

				Expect(mci).NotTo(BeAnExistingFile())
				Expect(filepath.Dir(mci)).NotTo(BeAnExistingFile())
			})
		})

		Context("with an invalid cached MultiCloudImage", func() {
			var mci string

			BeforeEach(func() {
				dir := filepath.Join(tempPath, "multi_cloud_images", "1", "2345678", "90")
				if err := os.MkdirAll(dir, 0700); err != nil {
					panic(err)
				}
				mci = filepath.Join(dir, "multi_cloud_image.yaml")
				if err := ioutil.WriteFile(mci, []byte(`Name: Ubuntu_18.04_x64
Description: Ubuntu 18.04 x64 LTS Bionic Beaver
Tags:
- rs_agent:mime_shellscript=https://rightlink.rightscale.com/rll/10.6.0/rightlink.boot.sh
- rs_agent:type=right_link_lite
Settings:
- Cloud: EC2 us-east-1
  Instance Type: c5.large
  Image: ami-abcdefg1234567890
`), 0600); err != nil {
					panic(err)
				}
				if err := ioutil.WriteFile(filepath.Join(dir, "item.json"), []byte(`{
  "multi_cloud_image": {
    "actions": [
      {
        "rel": "clone"
      }
    ],
    "description": "Ubuntu_18.04_x64",
    "links": [
      {
        "href": "/api/multi_cloud_images/2345678",
        "rel": "self"
      },
      {
        "href": "/api/multi_cloud_images/2345678/settings",
        "rel": "settings"
      },
      {
        "href": "/api/multi_cloud_images/2345678/matchers",
        "rel": "matchers"
      }
    ],
    "name": "Ubuntu 18.04 x64 LTS Bionic Beaver",
    "revision": 90
  },
  "md5": "64767c6b5a0c1b446c0cc27eb40cb832"
}
`), 0600); err != nil {
					panic(err)
				}
			})

			It("returns nil without an error and removes the cached MultiCloudImage directory", func() {
				cmci, err := cache.GetMultiCloudImage(1, "2345678", 90)
				Expect(err).NotTo(HaveOccurred())
				Expect(cmci).To(BeNil())

				Expect(mci).NotTo(BeAnExistingFile())
				Expect(filepath.Dir(mci)).NotTo(BeAnExistingFile())
			})
		})
	})

	Describe("GetMultiCloudImageFile", func() {
		It("gets a cached MultiCloudImage file path", func() {
			path, err := cache.GetMultiCloudImageFile(1, "2345678", 90)
			Expect(err).NotTo(HaveOccurred())
			Expect(filepath.Base(path)).To(Equal("multi_cloud_image.yaml"))

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

	Describe("PutMultiCloudImage", func() {
		BeforeEach(func() {
			dir := filepath.Join(tempPath, "multi_cloud_images", "1", "2345678", "90")
			if err := os.MkdirAll(dir, 0700); err != nil {
				panic(err)
			}
			if err := ioutil.WriteFile(filepath.Join(dir, "multi_cloud_image.yaml"), []byte(`Name: Ubuntu_18.04_x64
Description: Ubuntu 18.04 x64 LTS Bionic Beaver
Tags:
- rs_agent:mime_shellscript=https://rightlink.rightscale.com/rll/10.6.0/rightlink.boot.sh
- rs_agent:type=right_link_lite
Settings:
- Cloud: EC2 us-east-1
  Instance Type: c5.large
  Image: ami-1234567890abcdefg
`), 0600); err != nil {
				panic(err)
			}
		})

		It("writes an item JSON file", func() {
			mci := cm15.MultiCloudImage{
				Actions:     []map[string]string{{"rel": "clone"}},
				Description: "Ubuntu_18.04_x64",
				Links: []map[string]string{
					{
						"href": "/api/multi_cloud_images/2345678",
						"rel":  "self",
					},
					{
						"href": "/api/multi_cloud_images/2345678/settings",
						"rel":  "settings",
					},
					{
						"href": "/api/multi_cloud_images/2345678/matchers",
						"rel":  "matchers",
					},
				},
				Name:     "Ubuntu 18.04 x64 LTS Bionic Beaver",
				Revision: 90,
			}
			err := cache.PutMultiCloudImage(1, "2345678", 90, &mci)
			Expect(err).NotTo(HaveOccurred())

			item := filepath.Join(tempPath, "multi_cloud_images", "1", "2345678", "90", "item.json")
			Expect(item).To(BeARegularFile())
			Expect(ioutil.ReadFile(item)).To(MatchJSON(`{
  "multi_cloud_image": {
    "actions": [
      {
        "rel": "clone"
      }
    ],
    "description": "Ubuntu_18.04_x64",
    "links": [
      {
        "href": "/api/multi_cloud_images/2345678",
        "rel": "self"
      },
      {
        "href": "/api/multi_cloud_images/2345678/settings",
        "rel": "settings"
      },
      {
        "href": "/api/multi_cloud_images/2345678/matchers",
        "rel": "matchers"
      }
    ],
    "name": "Ubuntu 18.04 x64 LTS Bionic Beaver",
    "revision": 90
  },
  "md5": "64767c6b5a0c1b446c0cc27eb40cb832",
  "version": 1
}
`))
		})
	})
})
