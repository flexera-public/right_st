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
	"net"
	"strings"

	"github.com/rightscale/rsc/cm15"
	"github.com/rightscale/rsc/cm16"
	"github.com/rightscale/rsc/rsapi"
)

type Account struct {
	Host         string
	Id           int
	RefreshToken string `mapstructure:"refresh_token" yaml:"refresh_token,omitempty"`
	Username     string `mapstructure:"username" yaml:"username,omitempty"`
	Password     string `mapstructure:"password" yaml:"password,omitempty"`
	client15     *cm15.API
	client16     *cm16.API
}

const encryptedPrefix = "{ENCRYPTED}"

func (account *Account) Client15() (*cm15.API, error) {
	if account.client15 == nil {
		if err := account.validate(); err != nil {
			return nil, err
		}
		auth, err := account.Auth()
		if err != nil {
			return nil, err
		}
		account.client15 = cm15.New(account.Host, auth)
	}
	return account.client15, nil
}

func (account *Account) Client16() (*cm16.API, error) {
	if account.client16 == nil {
		if err := account.validate(); err != nil {
			return nil, err
		}
		auth, err := account.Auth()
		if err != nil {
			return nil, err
		}
		account.client16 = cm16.New(account.Host, auth)
	}
	return account.client16, nil
}

func (account *Account) Auth() (rsapi.Authenticator, error) {
	if account.RefreshToken != "" {
		return rsapi.NewOAuthAuthenticator(account.RefreshToken, account.Id), nil
	} else {
		password, err := account.DecryptPassword()
		if err != nil {
			return nil, err
		}
		return rsapi.NewBasicAuthenticator(account.Username, password, account.Id), nil
	}
}

func (account *Account) EncryptPassword(password string) error {
	p, err := Encrypt(password)
	if err != nil {
		return err
	}
	account.Password = encryptedPrefix + p
	return nil
}

func (account *Account) DecryptPassword() (string, error) {
	if strings.HasPrefix(account.Password, encryptedPrefix) {
		return Decrypt(strings.TrimPrefix(account.Password, encryptedPrefix))
	} else {
		return account.Password, nil
	}
}

func (account *Account) validate() error {
	if _, err := net.LookupIP(account.Host); err != nil {
		return fmt.Errorf("Invalid host name for account (host: %s, id: %d): %s", account.Host, account.Id, err)
	}
	return nil
}
