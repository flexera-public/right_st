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

	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/yaml.v2"
)

type ConfigViper struct {
	*viper.Viper
	Account  *Account
	Accounts map[string]*Account
}

var Config ConfigViper

func init() {
	Config.Viper = viper.New()
	Config.SetDefault("login", map[string]interface{}{"accounts": make(map[string]interface{})})
	Config.SetDefault("update", map[string]interface{}{"check": true})
	Config.SetEnvPrefix(app.Name)
	Config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	Config.AutomaticEnv()
}

func ReadConfig(configFile, account string) error {
	Config.SetConfigFile(configFile)
	err := Config.ReadInConfig()
	if err != nil {
		if _, ok := err.(*os.PathError); !(ok &&
			Config.IsSet("login.account.id") &&
			Config.IsSet("login.account.host") &&
			(Config.IsSet("login.account.refresh_token") ||
				(Config.IsSet("login.account.username") &&
					Config.IsSet("login.account.password")))) {
			return err
		}
	}

	err = Config.UnmarshalKey("login.accounts", &Config.Accounts)
	if err != nil {
		return fmt.Errorf("%s: %s", configFile, err)
	}

	if Config.IsSet("login.account.id") &&
		Config.IsSet("login.account.host") &&
		Config.IsSet("login.account.refresh_token") {
		Config.Account = &Account{
			Id:           Config.GetInt("login.account.id"),
			Host:         Config.GetString("login.account.host"),
			RefreshToken: Config.GetString("login.account.refresh_token"),
		}
	} else if Config.IsSet("login.account.id") &&
		Config.IsSet("login.account.host") &&
		Config.IsSet("login.account.username") &&
		Config.IsSet("login.account.password") {
		Config.Account = &Account{
			Id:       Config.GetInt("login.account.id"),
			Host:     Config.GetString("login.account.host"),
			Username: Config.GetString("login.account.username"),
			Password: Config.GetString("login.account.password"),
		}
	} else {
		var ok bool
		if account == "" {
			defaultAccount := Config.GetString("login.default_account")
			Config.Account, ok = Config.Accounts[strings.ToLower(defaultAccount)]
			if !ok {
				return fmt.Errorf("%s: could not find default account: %s", configFile, defaultAccount)
			}
		} else {
			Config.Account, ok = Config.Accounts[strings.ToLower(account)]
			if !ok {
				return fmt.Errorf("%s: could not find account: %s", configFile, account)
			}
		}
	}

	return nil
}

func (config *ConfigViper) GetAccount(id int, host string) (*Account, error) {
	for _, account := range config.Accounts {
		if account.Id == id && account.Host == host {
			return account, nil
		}
	}

	return nil, fmt.Errorf("Could not find account for account/host: %d %s", id, host)
}

// Obtain input via STDIN then print out to config file
// Example of config file
// login:
//   default_account: acct1
//   accounts:
//     acct1:
//       id: 67972
//       host: us-3.rightscale.com
//       refresh_token: abc123abc123abc123abc123abc123abc123abc1
//     acct2:
//       id: 60073
//       host: us-4.rightscale.com
//       refresh_token: zxy987zxy987zxy987zxy987xzy987zxy987xzy9
func (config *ConfigViper) SetAccount(name string, setDefault, setPassword bool, input io.Reader, output io.Writer) error {
	// if the default account isn't set we should set it to the account we are setting
	if !config.IsSet("login.default_account") {
		setDefault = true
	}

	// get the settings and specifically the login settings into a map we can manipulate and marshal to YAML unhindered
	// by the meddling of the Viper
	settings := config.AllSettings()
	if _, ok := settings["login"]; !ok {
		settings["login"] = map[string]interface{}{"accounts": make(map[string]interface{})}
	}
	loginSettings := settings["login"].(map[string]interface{})

	// set the default account if we want or need to
	if setDefault {
		loginSettings["default_account"] = name
	}

	// get the previous value for the named account if it exists and construct a new account to populate
	oldAccount, hasOldAccount := config.Accounts[name]
	newAccount := &Account{}

	// prompt for the account ID and use the old value if nothing is entered
	fmt.Fprint(output, "Account ID")
	if hasOldAccount {
		fmt.Fprintf(output, " (%d)", oldAccount.Id)
	}
	fmt.Fprint(output, ": ")
	fmt.Fscanln(input, &newAccount.Id)
	if hasOldAccount && newAccount.Id == 0 {
		newAccount.Id = oldAccount.Id
	}

	// prompt for the API endpoint host and use the old value if nothing is entered
	fmt.Fprint(output, "API endpoint host")
	if hasOldAccount {
		fmt.Fprintf(output, " (%s)", oldAccount.Host)
	}
	fmt.Fprint(output, ": ")
	fmt.Fscanln(input, &newAccount.Host)
	if hasOldAccount && newAccount.Host == "" {
		newAccount.Host = oldAccount.Host
	}

	if setPassword {
		// prompt for the username and use the old value if nothing is entered
		fmt.Fprintf(output, "Username")
		if hasOldAccount {
			fmt.Fprintf(output, " (%s)", oldAccount.Username)
		}
		fmt.Fprint(output, ": ")
		fmt.Fscanln(input, &newAccount.Username)
		if hasOldAccount && newAccount.Username == "" {
			newAccount.Username = oldAccount.Username
		}

		// prompt for the password and use the old value if nothing is entered
		fmt.Fprintf(output, "Password")
		if hasOldAccount {
			mask, err := oldAccount.MaskPassword()
			if err != nil {
				return err
			}
			fmt.Fprintf(output, " (%s)", mask)
		}
		fmt.Fprint(output, ": ")
		var password string
		if f, ok := input.(*os.File); ok && terminal.IsTerminal(int(f.Fd())) {
			b, err := terminal.ReadPassword(int(f.Fd()))
			if err != nil {
				return err
			}
			password = string(b)
		} else {
			fmt.Fscanln(input, &password)
		}
		if hasOldAccount && password == "" {
			newAccount.Password = oldAccount.Password
		} else {
			err := newAccount.EncryptPassword(password)
			if err != nil {
				return err
			}
		}
	} else {
		// prompt for the refresh token and use the old value if nothing is entered
		fmt.Fprint(output, "Refresh token")
		if hasOldAccount {
			fmt.Fprintf(output, " (%s)", oldAccount.RefreshToken)
		}
		fmt.Fprint(output, ": ")
		fmt.Fscanln(input, &newAccount.RefreshToken)
		if hasOldAccount && newAccount.RefreshToken == "" {
			newAccount.RefreshToken = oldAccount.RefreshToken
		}
	}

	// add the new account to the map of accounts overwriting any old value
	accounts := loginSettings["accounts"].(map[string]interface{})
	accounts[name] = newAccount

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

func (config *ConfigViper) ShowConfiguration(output io.Writer) error {
	// Check if config file exists
	if _, err := os.Stat(config.ConfigFileUsed()); err != nil {
		return err
	}

	settings := config.AllSettings()
	if _, ok := settings["login"]; !ok {
		settings["login"] = map[string]interface{}{"accounts": make(map[string]interface{})}
	}
	loginSettings := settings["login"].(map[string]interface{})
	accounts := loginSettings["accounts"].(map[string]interface{})

	for name, account := range config.Accounts {
		var err error
		account.Password, err = account.MaskPassword()
		if err != nil {
			return err
		}
		accounts[name] = account
	}

	yml, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = output.Write(yml)
	if err != nil {
		return err
	}

	return nil
}
