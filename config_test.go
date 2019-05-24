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
					if err := os.Setenv("RIGHT_ST_LOGIN_ACCOUNT_ID", "67890"); err != nil {
						panic(err)
					}
					if err := os.Setenv("RIGHT_ST_LOGIN_ACCOUNT_HOST", "us-4.rightscale.com"); err != nil {
						panic(err)
					}
					if err := os.Setenv("RIGHT_ST_LOGIN_ACCOUNT_REFRESH_TOKEN",
						"fedcba0987654321febcba0987654321fedcba09"); err != nil {
						panic(err)
					}
				})

				AfterEach(func() {
					if err := os.Unsetenv("RIGHT_ST_LOGIN_ACCOUNT_ID"); err != nil {
						panic(err)
					}
					if err := os.Unsetenv("RIGHT_ST_LOGIN_ACCOUNT_HOST"); err != nil {
						panic(err)
					}
					if err := os.Unsetenv("RIGHT_ST_LOGIN_ACCOUNT_REFRESH_TOKEN"); err != nil {
						panic(err)
					}
				})

				It("Loads the default account from the OS environment variables", func() {
					Expect(ReadConfig(nonexistentConfigFile, "")).To(Succeed())
					Expect(Config.Accounts).To(BeEmpty())
					Expect(Config.Account).To(Equal(&Account{
						Id:           67890,
						Host:         "us-4.rightscale.com",
						RefreshToken: "fedcba0987654321febcba0987654321fedcba09",
					}))
				})
			})

			Context("With a nonexistent parent directory for the config file", func() {
				BeforeEach(func() {
					os.RemoveAll(tempDir)
				})

				Describe("Set account", func() {
					Context("With refresh token", func() {
						It("Creates a config file with the set account", func() {
							Expect(ReadConfig(nonexistentConfigFile, "")).NotTo(Succeed())
							input := new(bytes.Buffer)
							fmt.Fprintln(input, 12345)
							fmt.Fprintln(input, "us-3.rightscale.com")
							fmt.Fprintln(input, "abcdef1234567890abcdef1234567890abcdef12")
							Expect(Config.SetAccount("production", false, false, input, buffer)).To(Succeed())
							Expect(buffer.Contents()).To(BeEquivalentTo("Account ID: API endpoint host: Refresh token: "))
							config, err := ioutil.ReadFile(nonexistentConfigFile)
							Expect(err).NotTo(HaveOccurred())
							Expect(string(config)).To(BeEquivalentTo(`login:
  accounts:
    production:
      host: us-3.rightscale.com
      id: 12345
      refresh_token: abcdef1234567890abcdef1234567890abcdef12
  default_account: production
update:
  check: true
`))
						})
					})
				})
			})
		})

		Context("With a bad account config file", func() {
			var badAccountConfigFile string

			BeforeEach(func() {
				badAccountConfigFile = filepath.Join(tempDir, ".right_st.yml")
				err := ioutil.WriteFile(badAccountConfigFile, []byte(`# Bad account config with an array instead of a dictionary
---
login:
  accounts:
    - production:
      host: us-3.rightscale.com
      id: 12345
      refresh_token: abcdef1234567890abcdef1234567890abcdef12
    - staging:
      host: us-4.rightscale.com
      id: 67890
      refresh_token: fedcba0987654321febcba0987654321fedcba09
  default_account: production
`), 0600)
				if err != nil {
					panic(err)
				}
			})

			It("Returns an error", func() {
				err := ReadConfig(badAccountConfigFile, "")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("With a missing default account in the config file", func() {
			var missingDefaultAccountConfigFile string

			BeforeEach(func() {
				missingDefaultAccountConfigFile = filepath.Join(tempDir, ".right_st.yml")
				err := ioutil.WriteFile(missingDefaultAccountConfigFile, []byte(`# Account config with missing default account
---
login:
  accounts:
    production:
      host: us-3.rightscale.com
      id: 12345
      refresh_token: abcdef1234567890abcdef1234567890abcdef12
    staging:
      host: us-4.rightscale.com
      id: 67890
      refresh_token: fedcba0987654321febcba0987654321fedcba09
  default_account: development
`), 0600)
				if err != nil {
					panic(err)
				}
			})

			It("Returns an error", func() {
				err := ReadConfig(missingDefaultAccountConfigFile, "")
				Expect(err).To(MatchError(missingDefaultAccountConfigFile + ": could not find default account: development"))
			})
		})

		Context("With a valid config file", func() {
			var configFile string

			BeforeEach(func() {
				configFile = filepath.Join(tempDir, ".right_st.yml")
				err := ioutil.WriteFile(configFile, []byte(`---
login:
  default_account: production
  accounts:
    production:
      host: us-3.rightscale.com
      id: 12345
      refresh_token: abcdef1234567890abcdef1234567890abcdef12
    staging:
      host: us-4.rightscale.com
      id: 67890
      refresh_token: fedcba0987654321febcba0987654321fedcba09
`), 0600)
				if err != nil {
					panic(err)
				}
			})

			It("Loads the accounts from the config file and sets the default account", func() {
				Expect(ReadConfig(configFile, "")).To(Succeed())
				Expect(Config.Accounts).To(Equal(map[string]*Account{
					"production": {
						Id:           12345,
						Host:         "us-3.rightscale.com",
						RefreshToken: "abcdef1234567890abcdef1234567890abcdef12",
					},
					"staging": {
						Id:           67890,
						Host:         "us-4.rightscale.com",
						RefreshToken: "fedcba0987654321febcba0987654321fedcba09",
					},
				}))
				Expect(Config.Account).To(Equal(&Account{
					Id:           12345,
					Host:         "us-3.rightscale.com",
					RefreshToken: "abcdef1234567890abcdef1234567890abcdef12",
				}))
			})

			It("Loads the accounts from the config file and set a specified account", func() {
				Expect(ReadConfig(configFile, "staging")).To(Succeed())
				Expect(Config.Accounts).To(Equal(map[string]*Account{
					"production": {
						Id:           12345,
						Host:         "us-3.rightscale.com",
						RefreshToken: "abcdef1234567890abcdef1234567890abcdef12",
					},
					"staging": {
						Id:           67890,
						Host:         "us-4.rightscale.com",
						RefreshToken: "fedcba0987654321febcba0987654321fedcba09",
					},
				}))
				Expect(Config.Account).To(Equal(&Account{
					Id:           67890,
					Host:         "us-4.rightscale.com",
					RefreshToken: "fedcba0987654321febcba0987654321fedcba09",
				}))
			})

			It("Returns an error when the config file does not contain the specified account", func() {
				err := ReadConfig(configFile, "development")
				Expect(err).To(MatchError(configFile + ": could not find account: development"))
			})

			Describe("Get account", func() {
				It("Gets an account with a specified account and host", func() {
					Expect(ReadConfig(configFile, "")).To(Succeed())
					account, err := Config.GetAccount(67890, "us-4.rightscale.com")
					Expect(err).NotTo(HaveOccurred())
					Expect(account).To(Equal(&Account{
						Id:           67890,
						Host:         "us-4.rightscale.com",
						RefreshToken: "fedcba0987654321febcba0987654321fedcba09",
					}))
				})

				It("Returns an error if the specified account and host are not in the configuration", func() {
					Expect(ReadConfig(configFile, "")).To(Succeed())
					account, err := Config.GetAccount(12345, "us-4.rightscale.com")
					Expect(err).To(MatchError("Could not find account for account/host: 12345 us-4.rightscale.com"))
					Expect(account).To(BeNil())
				})
			})

			Context("With OS environment variables", func() {
				BeforeEach(func() {
					if err := os.Setenv("RIGHT_ST_LOGIN_ACCOUNT_ID", "67890"); err != nil {
						panic(err)
					}
					if err := os.Setenv("RIGHT_ST_LOGIN_ACCOUNT_HOST", "us-4.rightscale.com"); err != nil {
						panic(err)
					}
					if err := os.Setenv("RIGHT_ST_LOGIN_ACCOUNT_REFRESH_TOKEN",
						"fedcba0987654321febcba0987654321fedcba09"); err != nil {
						panic(err)
					}
				})

				AfterEach(func() {
					if err := os.Unsetenv("RIGHT_ST_LOGIN_ACCOUNT_ID"); err != nil {
						panic(err)
					}
					if err := os.Unsetenv("RIGHT_ST_LOGIN_ACCOUNT_HOST"); err != nil {
						panic(err)
					}
					if err := os.Unsetenv("RIGHT_ST_LOGIN_ACCOUNT_REFRESH_TOKEN"); err != nil {
						panic(err)
					}
				})

				It("Loads the default account from the OS environment variables", func() {
					Expect(ReadConfig(configFile, "")).To(Succeed())
					Expect(Config.Accounts).To(Equal(map[string]*Account{
						"production": {
							Id:           12345,
							Host:         "us-3.rightscale.com",
							RefreshToken: "abcdef1234567890abcdef1234567890abcdef12",
						},
						"staging": {
							Id:           67890,
							Host:         "us-4.rightscale.com",
							RefreshToken: "fedcba0987654321febcba0987654321fedcba09",
						},
					}))
					Expect(Config.Account).To(Equal(&Account{
						Id:           67890,
						Host:         "us-4.rightscale.com",
						RefreshToken: "fedcba0987654321febcba0987654321fedcba09",
					}))
				})
			})

			Describe("Set account", func() {
				Context("With refresh token", func() {
					It("Updates the config file with the new account", func() {
						Expect(ReadConfig(configFile, "")).To(Succeed())
						input := new(bytes.Buffer)
						fmt.Fprintln(input, 54321)
						fmt.Fprintln(input, "us-4.rightscale.com")
						fmt.Fprintln(input, "21fedcba0987654321fedcba0987654321fedcba")
						Expect(Config.SetAccount("testing", false, false, input, buffer)).To(Succeed())
						Expect(buffer.Contents()).To(BeEquivalentTo("Account ID: " + "API endpoint host: " +
							"Refresh token: "))
						config, err := ioutil.ReadFile(configFile)
						Expect(err).NotTo(HaveOccurred())
						Expect(string(config)).To(BeEquivalentTo(`login:
  accounts:
    production:
      host: us-3.rightscale.com
      id: 12345
      refresh_token: abcdef1234567890abcdef1234567890abcdef12
    staging:
      host: us-4.rightscale.com
      id: 67890
      refresh_token: fedcba0987654321febcba0987654321fedcba09
    testing:
      host: us-4.rightscale.com
      id: 54321
      refresh_token: 21fedcba0987654321fedcba0987654321fedcba
  default_account: production
update:
  check: true
`))
					})

					It("Updates the default account and uses defaults when modifying an existing account", func() {
						Expect(ReadConfig(configFile, "")).To(Succeed())
						Expect(Config.SetAccount("staging", true, false, new(bytes.Buffer), buffer)).To(Succeed())
						Expect(buffer.Contents()).To(BeEquivalentTo("Account ID (67890): " +
							"API endpoint host (us-4.rightscale.com): " +
							"Refresh token (fedcba0987654321febcba0987654321fedcba09): "))
						config, err := ioutil.ReadFile(configFile)
						Expect(err).NotTo(HaveOccurred())
						Expect(string(config)).To(BeEquivalentTo(`login:
  accounts:
    production:
      host: us-3.rightscale.com
      id: 12345
      refresh_token: abcdef1234567890abcdef1234567890abcdef12
    staging:
      host: us-4.rightscale.com
      id: 67890
      refresh_token: fedcba0987654321febcba0987654321fedcba09
  default_account: staging
update:
  check: true
`))
					})
				})
			})

			Describe("Show configuration", func() {
				It("Prints the configuration", func() {
					Expect(ReadConfig(configFile, "")).To(Succeed())
					Expect(Config.ShowConfiguration(buffer)).To(Succeed())
					Expect(buffer.Contents()).To(BeEquivalentTo(`login:
  accounts:
    production:
      host: us-3.rightscale.com
      id: 12345
      refresh_token: abcdef1234567890abcdef1234567890abcdef12
    staging:
      host: us-4.rightscale.com
      id: 67890
      refresh_token: fedcba0987654321febcba0987654321fedcba09
  default_account: production
update:
  check: true
`))
				})
			})
		})
	})
})
