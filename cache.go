package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type (
	Cache interface {
		GetServerTemplateDir(account, id string, revision uint) (path string, err error)
		GetRightScriptDir(account, id string, revision uint) (path string, err error)
		GetMultiCloudImageDir(account, id string, revision uint) (path string, err error)
	}

	cache struct {
		stPath  string
		rsPath  string
		mciPath string
	}
)

func NewCache(path string) (Cache, error) {
	c := &cache{
		stPath:  filepath.Join(path, "servertemplates"),
		rsPath:  filepath.Join(path, "rightscripts"),
		mciPath: filepath.Join(path, "multicloudimages"),
	}

	if err := os.MkdirAll(c.stPath, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(c.rsPath, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(c.mciPath, 0755); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *cache) GetServerTemplateDir(account, id string, revision uint) (string, error) {
	return cacheCheck(c.stPath, account, id, revision)
}

func (c *cache) GetRightScriptDir(account, id string, revision uint) (string, error) {
	return cacheCheck(c.rsPath, account, id, revision)
}

func (c *cache) GetMultiCloudImageDir(account, id string, revision uint) (string, error) {
	return cacheCheck(c.mciPath, account, id, revision)
}

func cacheCheck(path, account, id string, revision uint) (string, error) {
	if revision == 0 {
		return "", errors.New("cannot cache HEAD revision")
	}

	path = filepath.Join(path, account, id, fmt.Sprintf("%v", revision))
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", err
	}
	return path, nil
}
