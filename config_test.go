// The MIT License (MIT)
//
// Copyright (c) 2015 Douglas Thrift
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/rightscale/right_st"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Config", func() {
	It("Gets the default config file", func() {
		configFile := DefaultConfigFile()
		Expect(filepath.Base(configFile)).To(Equal(".right_st.yml"))
		Expect(filepath.IsAbs(configFile)).To(BeTrue())
	})

	Describe("Read config", func() {
		var (
			tempDir string
			buffer  *gbytes.Buffer
		)

		BeforeEach(func() {
			var err error
			tempDir, err = ioutil.TempDir("", "config")
			if err != nil {
				panic(err)
			}
			buffer = gbytes.NewBuffer()
		})

		AfterEach(func() {
			os.RemoveAll(tempDir)
		})

		Context("With a nonexistent config file", func() {
			var nonexistentConfigFile string

			BeforeEach(func() {
				nonexistentConfigFile = filepath.Join(tempDir, ".right_st.yml")
			})

			It("Returns an error", func() {
				err := ReadConfig(nonexistentConfigFile, "")
				Expect(err).To(HaveOccurred())
			})

			Context("With OS environment variables set", func() {
				BeforeEach(func() {
					if err := os.Setenv("RIGHT_ST_LOGIN_ENVIRONMENT_ACCOUNT", "67890"); err != nil {
						panic(err)
					}
					if err := os.Setenv("RIGHT_ST_LOGIN_ENVIRONMENT_HOST", "us-4.rightscale.com"); err != nil {
						panic(err)
					}
					if err := os.Setenv("RIGHT_ST_LOGIN_ENVIRONMENT_REFRESH_TOKEN",
						"fedcba0987654321febcba0987654321fedcba09"); err != nil {
						panic(err)
					}
				})

				AfterEach(func() {
					if err := os.Unsetenv("RIGHT_ST_LOGIN_ENVIRONMENT_ACCOUNT"); err != nil {
						panic(err)
					}
					if err := os.Unsetenv("RIGHT_ST_LOGIN_ENVIRONMENT_HOST"); err != nil {
						panic(err)
					}
					if err := os.Unsetenv("RIGHT_ST_LOGIN_ENVIRONMENT_REFRESH_TOKEN"); err != nil {
						panic(err)
					}
				})

				It("Loads the default environment from the OS environment variables", func() {
					Expect(ReadConfig(nonexistentConfigFile, "")).To(Succeed())
					Expect(Config.Environments).To(BeEmpty())
					Expect(Config.Environment).To(Equal(&Environment{
						Account:      67890,
						Host:         "us-4.rightscale.com",
						RefreshToken: "fedcba0987654321febcba0987654321fedcba09",
					}))
				})
			})

			Context("With a nonexistent parent directory for the config file", func() {
				BeforeEach(func() {
					os.RemoveAll(tempDir)
				})

				Describe("Set environment", func() {
					It("Creates a config file with the set environment", func() {
						Expect(ReadConfig(nonexistentConfigFile, "")).NotTo(Succeed())
						input := new(bytes.Buffer)
						fmt.Fprintln(input, 12345)
						fmt.Fprintln(input, "us-3.rightscale.com")
						fmt.Fprintln(input, "abcdef1234567890abcdef1234567890abcdef12")
						Expect(Config.SetEnvironment("production", false, input, buffer)).To(Succeed())
						Expect(buffer.Contents()).To(BeEquivalentTo("Account ID: API endpoint host: Refresh token: "))
						config, err := ioutil.ReadFile(nonexistentConfigFile)
						Expect(err).NotTo(HaveOccurred())
						Expect(config).To(BeEquivalentTo(`login:
  default_environment: production
  environments:
    production:
      account: 12345
      host: us-3.rightscale.com
      refresh_token: abcdef1234567890abcdef1234567890abcdef12
update:
  check: true
`))
					})
				})
			})
		})

		Context("With a bad environment config file", func() {
			var badEnvironmentConfigFile string

			BeforeEach(func() {
				badEnvironmentConfigFile = filepath.Join(tempDir, ".right_st.yml")
				err := ioutil.WriteFile(badEnvironmentConfigFile, []byte(`# Bad environment config with an array instead of a dictionary
---
login:
  default_environment: production
  environments:
    - production:
      account: 12345
      host: us-3.rightscale.com
      refresh_token: abcdef1234567890abcdef1234567890abcdef12
    - staging:
      account: 67890
      host: us-4.rightscale.com
      refresh_token: fedcba0987654321febcba0987654321fedcba09
`), 0600)
				if err != nil {
					panic(err)
				}
			})

			It("Returns an error", func() {
				err := ReadConfig(badEnvironmentConfigFile, "")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("With a missing default environment in the config file", func() {
			var missingDefaultEnvironmentConfigFile string

			BeforeEach(func() {
				missingDefaultEnvironmentConfigFile = filepath.Join(tempDir, ".right_st.yml")
				err := ioutil.WriteFile(missingDefaultEnvironmentConfigFile, []byte(`# Environment config with missing default environment
---
login:
  default_environment: development
  environments:
    production:
      account: 12345
      host: us-3.rightscale.com
      refresh_token: abcdef1234567890abcdef1234567890abcdef12
    staging:
      account: 67890
      host: us-4.rightscale.com
      refresh_token: fedcba0987654321febcba0987654321fedcba09
`), 0600)
				if err != nil {
					panic(err)
				}
			})

			It("Returns an error", func() {
				err := ReadConfig(missingDefaultEnvironmentConfigFile, "")
				Expect(err).To(MatchError(missingDefaultEnvironmentConfigFile + ": could not find default environment: development"))
			})
		})

		Context("With a valid config file", func() {
			var configFile string

			BeforeEach(func() {
				configFile = filepath.Join(tempDir, ".right_st.yml")
				err := ioutil.WriteFile(configFile, []byte(`---
login:
  default_environment: production
  environments:
    production:
      account: 12345
      host: us-3.rightscale.com
      refresh_token: abcdef1234567890abcdef1234567890abcdef12
    staging:
      account: 67890
      host: us-4.rightscale.com
      refresh_token: fedcba0987654321febcba0987654321fedcba09
`), 0600)
				if err != nil {
					panic(err)
				}
			})

			It("Loads the environments from the config file and sets the default environment", func() {
				Expect(ReadConfig(configFile, "")).To(Succeed())
				Expect(Config.Environments).To(Equal(map[string]*Environment{
					"production": {
						Account:      12345,
						Host:         "us-3.rightscale.com",
						RefreshToken: "abcdef1234567890abcdef1234567890abcdef12",
					},
					"staging": {
						Account:      67890,
						Host:         "us-4.rightscale.com",
						RefreshToken: "fedcba0987654321febcba0987654321fedcba09",
					},
				}))
				Expect(Config.Environment).To(Equal(&Environment{
					Account:      12345,
					Host:         "us-3.rightscale.com",
					RefreshToken: "abcdef1234567890abcdef1234567890abcdef12",
				}))
			})

			It("Loads the environments from the config file and set a specified environment", func() {
				Expect(ReadConfig(configFile, "staging")).To(Succeed())
				Expect(Config.Environments).To(Equal(map[string]*Environment{
					"production": {
						Account:      12345,
						Host:         "us-3.rightscale.com",
						RefreshToken: "abcdef1234567890abcdef1234567890abcdef12",
					},
					"staging": {
						Account:      67890,
						Host:         "us-4.rightscale.com",
						RefreshToken: "fedcba0987654321febcba0987654321fedcba09",
					},
				}))
				Expect(Config.Environment).To(Equal(&Environment{
					Account:      67890,
					Host:         "us-4.rightscale.com",
					RefreshToken: "fedcba0987654321febcba0987654321fedcba09",
				}))
			})

			It("Returns an error when the config file does not contain the specified environment", func() {
				err := ReadConfig(configFile, "development")
				Expect(err).To(MatchError(configFile + ": could not find environment: development"))
			})

			Describe("Get environment", func() {
				It("Gets an environment with a specified account and host", func() {
					Expect(ReadConfig(configFile, "")).To(Succeed())
					environment, err := Config.GetEnvironment(67890, "us-4.rightscale.com")
					Expect(err).NotTo(HaveOccurred())
					Expect(environment).To(Equal(&Environment{
						Account:      67890,
						Host:         "us-4.rightscale.com",
						RefreshToken: "fedcba0987654321febcba0987654321fedcba09",
					}))
				})

				It("Returns an error if the specified account and host are not in the configuration", func() {
					Expect(ReadConfig(configFile, "")).To(Succeed())
					environment, err := Config.GetEnvironment(12345, "us-4.rightscale.com")
					Expect(err).To(MatchError("Could not find environment for account/host: 12345 us-4.rightscale.com"))
					Expect(environment).To(BeNil())
				})
			})

			Context("With OS environment variables", func() {
				BeforeEach(func() {
					if err := os.Setenv("RIGHT_ST_LOGIN_ENVIRONMENT_ACCOUNT", "67890"); err != nil {
						panic(err)
					}
					if err := os.Setenv("RIGHT_ST_LOGIN_ENVIRONMENT_HOST", "us-4.rightscale.com"); err != nil {
						panic(err)
					}
					if err := os.Setenv("RIGHT_ST_LOGIN_ENVIRONMENT_REFRESH_TOKEN",
						"fedcba0987654321febcba0987654321fedcba09"); err != nil {
						panic(err)
					}
				})

				AfterEach(func() {
					if err := os.Unsetenv("RIGHT_ST_LOGIN_ENVIRONMENT_ACCOUNT"); err != nil {
						panic(err)
					}
					if err := os.Unsetenv("RIGHT_ST_LOGIN_ENVIRONMENT_HOST"); err != nil {
						panic(err)
					}
					if err := os.Unsetenv("RIGHT_ST_LOGIN_ENVIRONMENT_REFRESH_TOKEN"); err != nil {
						panic(err)
					}
				})

				It("Loads the default environment from the OS environment variables", func() {
					Expect(ReadConfig(configFile, "")).To(Succeed())
					Expect(Config.Environments).To(Equal(map[string]*Environment{
						"production": {
							Account:      12345,
							Host:         "us-3.rightscale.com",
							RefreshToken: "abcdef1234567890abcdef1234567890abcdef12",
						},
						"staging": {
							Account:      67890,
							Host:         "us-4.rightscale.com",
							RefreshToken: "fedcba0987654321febcba0987654321fedcba09",
						},
					}))
					Expect(Config.Environment).To(Equal(&Environment{
						Account:      67890,
						Host:         "us-4.rightscale.com",
						RefreshToken: "fedcba0987654321febcba0987654321fedcba09",
					}))
				})
			})

			Describe("Set environment", func() {
				It("Updates the config file with the new environment", func() {
					Expect(ReadConfig(configFile, "")).To(Succeed())
					input := new(bytes.Buffer)
					fmt.Fprintln(input, 54321)
					fmt.Fprintln(input, "us-4.rightscale.com")
					fmt.Fprintln(input, "21fedcba0987654321fedcba0987654321fedcba")
					Expect(Config.SetEnvironment("testing", false, input, buffer)).To(Succeed())
					Expect(buffer.Contents()).To(BeEquivalentTo("Account ID: " + "API endpoint host: " +
						"Refresh token: "))
					config, err := ioutil.ReadFile(configFile)
					Expect(err).NotTo(HaveOccurred())
					Expect(config).To(BeEquivalentTo(`login:
  default_environment: production
  environments:
    production:
      account: 12345
      host: us-3.rightscale.com
      refresh_token: abcdef1234567890abcdef1234567890abcdef12
    staging:
      account: 67890
      host: us-4.rightscale.com
      refresh_token: fedcba0987654321febcba0987654321fedcba09
    testing:
      account: 54321
      host: us-4.rightscale.com
      refresh_token: 21fedcba0987654321fedcba0987654321fedcba
update:
  check: true
`))
				})

				It("Updates the default environment and uses defaults when modifying an existing environment", func() {
					Expect(ReadConfig(configFile, "")).To(Succeed())
					Expect(Config.SetEnvironment("staging", true, new(bytes.Buffer), buffer)).To(Succeed())
					Expect(buffer.Contents()).To(BeEquivalentTo("Account ID (67890): " +
						"API endpoint host (us-4.rightscale.com): " +
						"Refresh token (fedcba0987654321febcba0987654321fedcba09): "))
					config, err := ioutil.ReadFile(configFile)
					Expect(err).NotTo(HaveOccurred())
					Expect(config).To(BeEquivalentTo(`login:
  default_environment: staging
  environments:
    production:
      account: 12345
      host: us-3.rightscale.com
      refresh_token: abcdef1234567890abcdef1234567890abcdef12
    staging:
      account: 67890
      host: us-4.rightscale.com
      refresh_token: fedcba0987654321febcba0987654321fedcba09
update:
  check: true
`))
				})
			})

			Describe("List configuration", func() {
				It("Prints the configuration", func() {
					Expect(ReadConfig(configFile, "")).To(Succeed())
					Expect(Config.ListConfiguration(buffer)).To(Succeed())
					Expect(buffer.Contents()).To(BeEquivalentTo(`login:
  default_environment: production
  environments:
    production:
      account: 12345
      host: us-3.rightscale.com
      refresh_token: abcdef1234567890abcdef1234567890abcdef12
    staging:
      account: 67890
      host: us-4.rightscale.com
      refresh_token: fedcba0987654321febcba0987654321fedcba09
update:
  check: true
`))
				})
			})
		})
	})
})
