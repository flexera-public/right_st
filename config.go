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
	"io/ioutil"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	*viper.Viper
	environment  *Environment
	environments map[string]*Environment
}

var config Config

func init() {
	config.Viper = viper.New()
	config.SetDefault("update.check", true)
}

func readConfig(configFile, environment string) error {
	config.SetConfigFile(configFile)
	err := config.ReadInConfig()
	if err != nil {
		return err
	}

	err = config.UnmarshalKey("login.environments", &config.environments)
	if err != nil {
		return fmt.Errorf("%s: %s", configFile, err)
	}

	var ok bool
	if environment == "" {
		defaultEnvironment := config.GetString("login.default_environment")
		config.environment, ok = config.environments[defaultEnvironment]
		if !ok {
			return fmt.Errorf("%s: could not find default environment: %s", configFile, defaultEnvironment)
		}
	} else {
		config.environment, ok = config.environments[environment]
		if !ok {
			return fmt.Errorf("%s: could not find environment: %s", configFile, environment)
		}
	}

	return nil
}

func (config *Config) getEnvironment(account int, host string) (*Environment, error) {
	for _, environment := range config.environments {
		if environment.Account == account && environment.Host == host {
			return environment, nil
		}
	}

	return nil, fmt.Errorf("Error finding environment for account/host: %d %s", account, host)
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
func generateConfig(configFile, EnvironmentName string) error {

	// Create basic config file shell if it does not exist
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Printf("file %s does not exist - creating new file\n", configFile)
		NewConfig := []byte("login:\n")
		NewConfig = append(NewConfig, "  default_environment: "+EnvironmentName+"\n"...)
		NewConfig = append(NewConfig, "  environments:\n"...)
		NewConfig = append(NewConfig, "    "+EnvironmentName+":\n"...)
		NewConfig = append(NewConfig, "      account: \"\"\n"...)
		NewConfig = append(NewConfig, "      host: \"\"\n"...)
		NewConfig = append(NewConfig, "      refresh_token: \"\"\n"...)

		// Write config file
		err := ioutil.WriteFile(configFile, NewConfig, 0600)
		if err != nil {
			return err
		}
	}

	genconfig := viper.New()
	genconfig.SetConfigFile(configFile)

	// Read config file if it exists and obtain info if environment exists
	err := genconfig.ReadInConfig()
	if err != nil {
		return err
	}

	// Grab list of all environments currently in config file
	EnvironmentList := genconfig.GetStringMapString("login.environments")
	// If EnvironmentName not in list, add it. Viper does not create it
	// even if populating items in the environment.
	if !genconfig.IsSet("login.environments." + EnvironmentName) {
		EnvironmentList[EnvironmentName] = ""
	}

	// Populate variables specific to the environment to be used as defaults
	AccountNum := config.GetString("login.environments." + EnvironmentName + ".account")
	HostEndPoint := config.GetString("login.environments." + EnvironmentName + ".host")
	RefreshToken := config.GetString("login.environments." + EnvironmentName + ".refresh_token")

	// Prompt for updated/new info and set it
	fmt.Printf("Account Number (%s): ", AccountNum)
	fmt.Scanln(&AccountNum)
	genconfig.Set("login.environments."+EnvironmentName+".account", AccountNum)

	fmt.Printf("Host End Point (%s): ", HostEndPoint)
	fmt.Scanln(&HostEndPoint)
	genconfig.Set("login.environments."+EnvironmentName+".host", HostEndPoint)

	fmt.Printf("Refresh Token: (%s): ", RefreshToken)
	fmt.Scanln(&RefreshToken)
	genconfig.Set("login.environments."+EnvironmentName+".refresh_token", RefreshToken)

	// Build config file
	NewConfig := []byte("login:\n")
	NewConfig = append(NewConfig, "  default_environment: "+genconfig.GetString("login.default_environment")+"\n"...)
	NewConfig = append(NewConfig, "  environments:\n"...)
	for EnvName := range EnvironmentList {
		NewConfig = append(NewConfig, "    "+EnvName+":\n"...)
		NewConfig = append(NewConfig, "      account: "+genconfig.GetString("login.environments."+EnvName+".account")+"\n"...)
		NewConfig = append(NewConfig, "      host: "+genconfig.GetString("login.environments."+EnvName+".host")+"\n"...)
		NewConfig = append(NewConfig, "      refresh_token: "+genconfig.GetString("login.environments."+EnvName+".refresh_token")+"\n"...)
	}

	// Write config file
	err = ioutil.WriteFile(configFile, NewConfig, 0600)
	if err != nil {
		return err
	}
	return nil
}
