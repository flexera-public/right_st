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

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-yaml/yaml"
	"github.com/spf13/viper"
)

type ConfigViper struct {
	*viper.Viper
	Environment  *Environment
	Environments map[string]*Environment
}

var Config ConfigViper

func init() {
	Config.Viper = viper.New()
	Config.SetDefault("login", map[interface{}]interface{}{"environments": make(map[interface{}]interface{})})
	Config.SetDefault("update", map[string]interface{}{"check": true})
	Config.SetEnvPrefix(app.Name)
	Config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	Config.AutomaticEnv()
}

func ReadConfig(configFile, environment string) error {
	Config.SetConfigFile(configFile)
	err := Config.ReadInConfig()
	if err != nil {
		if _, ok := err.(*os.PathError); !(ok &&
			Config.IsSet("login.environment.account") &&
			Config.IsSet("login.environment.host") &&
			Config.IsSet("login.environment.refresh_token")) {
			return err
		}
	}

	err = Config.UnmarshalKey("login.environments", &Config.Environments)
	if err != nil {
		return fmt.Errorf("%s: %s", configFile, err)
	}

	if Config.IsSet("login.environment.account") &&
		Config.IsSet("login.environment.host") &&
		Config.IsSet("login.environment.refresh_token") {
		Config.Environment = &Environment{
			Account:      Config.GetInt("login.environment.account"),
			Host:         Config.GetString("login.environment.host"),
			RefreshToken: Config.GetString("login.environment.refresh_token"),
		}
	} else {
		var ok bool
		if environment == "" {
			defaultEnvironment := Config.GetString("login.default_environment")
			Config.Environment, ok = Config.Environments[defaultEnvironment]
			if !ok {
				return fmt.Errorf("%s: could not find default environment: %s", configFile, defaultEnvironment)
			}
		} else {
			Config.Environment, ok = Config.Environments[environment]
			if !ok {
				return fmt.Errorf("%s: could not find environment: %s", configFile, environment)
			}
		}
	}

	return nil
}

func (config *ConfigViper) GetEnvironment(account int, host string) (*Environment, error) {
	for _, environment := range config.Environments {
		if environment.Account == account && environment.Host == host {
			return environment, nil
		}
	}

	return nil, fmt.Errorf("Could not find environment for account/host: %d %s", account, host)
}

// Obtain input via STDIN then print out to config file
// Example of config file
// login:
//   default_environment: acct1
//   environments:
//     acct1:
//       account: 67972
//       host: us-3.rightscale.com
//       refresh_token: abc123abc123abc123abc123abc123abc123abc1
//     acct2:
//       account: 60073
//       host: us-4.rightscale.com
//       refresh_token: zxy987zxy987zxy987zxy987xzy987zxy987xzy9
func (config *ConfigViper) SetEnvironment(name string, setDefault bool, input io.Reader, output io.Writer) error {
	// if the default environment isn't set we should set it to the environment we are setting
	if !config.IsSet("login.default_environment") {
		setDefault = true
	}

	// get the settings and specifically the login settings into a map we can manipulate and marshal to YAML unhindered
	// by the meddling of the Viper
	settings := config.AllSettings()
	loginSettings := settings["login"].(map[interface{}]interface{})

	// set the default environment if we want or need to
	if setDefault {
		loginSettings["default_environment"] = name
	}

	// get the previous value for the named environment if it exists and construct a new environment to populate
	oldEnvironment, ok := config.Environments[name]
	newEnvironment := &Environment{}

	// prompt for the account ID and use the old value if nothing is entered
	fmt.Fprint(output, "Account ID")
	if ok {
		fmt.Fprintf(output, " (%d)", oldEnvironment.Account)
	}
	fmt.Fprint(output, ": ")
	fmt.Fscanln(input, &newEnvironment.Account)
	if ok && newEnvironment.Account == 0 {
		newEnvironment.Account = oldEnvironment.Account
	}

	// prompt for the API endpoint host and use the old value if nothing is entered
	fmt.Fprint(output, "API endpoint host")
	if ok {
		fmt.Fprintf(output, " (%s)", oldEnvironment.Host)
	}
	fmt.Fprint(output, ": ")
	fmt.Fscanln(input, &newEnvironment.Host)
	if ok && newEnvironment.Host == "" {
		newEnvironment.Host = oldEnvironment.Host
	}

	// prompt for the refresh token and use the old value if nothing is entered
	fmt.Fprint(output, "Refresh token")
	if ok {
		fmt.Fprintf(output, " (%s)", oldEnvironment.RefreshToken)
	}
	fmt.Fprint(output, ": ")
	fmt.Fscanln(input, &newEnvironment.RefreshToken)
	if ok && newEnvironment.RefreshToken == "" {
		newEnvironment.RefreshToken = oldEnvironment.RefreshToken
	}

	// add the new environment to the map of environments overwriting any old value
	environments := loginSettings["environments"].(map[interface{}]interface{})
	environments[name] = newEnvironment

	// render the settings map as YAML
	yml, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}

	configPath := config.ConfigFileUsed()
	// back up the current config file before writing a new one or if one does not exist, make sure the directory exists
	if err := os.Rename(configPath, configPath+".bak"); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// create a new config file which only the current user can read or write
	configFile, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer configFile.Close()

	// write the YAML into the config file
	if _, err := configFile.Write(yml); err != nil {
		return err
	}

	return nil
}

func (config *ConfigViper) ListConfiguration(output io.Writer) error {
	// Check if config file exists
	if _, err := os.Stat(config.ConfigFileUsed()); err != nil {
		return err
	}

	yml, err := yaml.Marshal(config.AllSettings())
	if err != nil {
		return err
	}
	output.Write(yml)

	return nil
}
