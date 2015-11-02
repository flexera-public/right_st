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
	"gopkg.in/rightscale/rsc.v4/cm15"
	"gopkg.in/rightscale/rsc.v4/cm16"
	"gopkg.in/rightscale/rsc.v4/rsapi"
)

type Environment struct {
	Account      int
	Host         string
	RefreshToken string `mapstructure:"refresh_token"`
	client15     *cm15.API
	client16     *cm16.API
}

func (environment *Environment) Client15() *cm15.API {
	if environment.client15 == nil {
		auth := rsapi.NewOAuthAuthenticator(environment.RefreshToken, environment.Account)
		environment.client15 = cm15.New(environment.Host, auth)
	}
	return environment.client15
}

func (environment *Environment) Client16() *cm16.API {
	if environment.client16 == nil {
		auth := rsapi.NewOAuthAuthenticator(environment.RefreshToken, environment.Account)
		environment.client16 = cm16.New(environment.Host, auth)
	}
	return environment.client16
}
