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

	"github.com/rightscale/rsc/cm15"
	"github.com/rightscale/rsc/cm16"
	"github.com/rightscale/rsc/rsapi"
)

type Account struct {
	Host         string
	Id           int
	RefreshToken string `mapstructure:"refresh_token" yaml:"refresh_token"`
	client15     *cm15.API
	client16     *cm16.API
}

func (account *Account) Client15() (*cm15.API, error) {
	if err := account.validate(); err != nil {
		return nil, err
	}
	if account.client15 == nil {
		auth := rsapi.NewOAuthAuthenticator(account.RefreshToken, account.Id)
		account.client15 = cm15.New(account.Host, auth)
	}
	return account.client15, nil
}

func (account *Account) Client16() (*cm16.API, error) {
	if err := account.validate(); err != nil {
		return nil, err
	}
	if account.client16 == nil {
		auth := rsapi.NewOAuthAuthenticator(account.RefreshToken, account.Id)
		account.client16 = cm16.New(account.Host, auth)
	}
	return account.client16, nil
}

func (account *Account) validate() error {
	if _, err := net.LookupIP(account.Host); err != nil {
		return fmt.Errorf("Invalid host name for account (host: %s, id: %d): %s", account.Host, account.Id, err)
	}
	return nil
}
