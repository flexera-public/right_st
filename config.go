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

	"github.com/spf13/viper"
)

type Config struct {
	*viper.Viper
	environment  *Environment
	environments map[string]*Environment
}

var config Config

func readConfig(configFile, environment string) error {
	config.Viper = viper.New()
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
