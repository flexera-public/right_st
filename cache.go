package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/rightscale/rsc/cm15"
)

type (
	Cache interface {
		GetServerTemplate(account int, id string, revision int) (CachedServerTemplate, bool, error)
		GetServerTemplateDir(account int, id string, revision int) (path string, err error)
		GetServerTemplateFile(account int, id string, revision int) (path string, err error)
		PutServerTemplate(account int, id string, revision int, st *cm15.ServerTemplate) error

		GetRightScript(account int, id string, revison int) (CachedRightScript, bool, error)
		GetRightScriptDir(account int, id string, revision int) (path string, err error)
		GetRightScriptFile(account int, id string, revision int) (path string, err error)
		PutRightScript(account int, id string, revision int, rs *cm15.RightScript, attachments []string) error

		GetMultiCloudImage(account int, id string, revision int) (CachedMultiCloudImage, bool, error)
		GetMultiCloudImageDir(account int, id string, revision int) (path string, err error)
		GetMultiCloudImageFile(account int, id string, revision int) (path string, err error)
		PutMultiCloudImage(account int, id string, revision int, mci *cm15.MultiCloudImage) error
	}

	cache struct {
		stPath  string
		rsPath  string
		mciPath string
	}

	CachedServerTemplate struct {
		*cm15.ServerTemplate `json:"server_template"`
		File                 string `json:"-"`
		MD5Sum               string `json:"md5"`
	}

	CachedRightScript struct {
		*cm15.RightScript `json:"right_script"`
		File              string            `json:"-"`
		MD5Sum            string            `json:"md5"`
		Attachments       map[string]string `json:"attachments,omitempty"`
	}

	CachedMultiCloudImage struct {
		*cm15.MultiCloudImage `json:"multi_cloud_image"`
		File                  string `json:"-"`
		MD5Sum                string `json:"md5"`
	}
)

func NewCache(path string) (Cache, error) {
	c := &cache{
		stPath:  filepath.Join(path, "server_templates"),
		rsPath:  filepath.Join(path, "right_scripts"),
		mciPath: filepath.Join(path, "multi_cloud_images"),
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

func (c *cache) GetServerTemplate(account int, id string, revision int) (mci CachedServerTemplate, ok bool, err error) {
	return
}

func (c *cache) GetServerTemplateDir(account int, id string, revision int) (string, error) {
	return cacheCheck(c.stPath, account, id, revision)
}

func (c *cache) GetServerTemplateFile(account int, id string, revision int) (string, error) {
	dir, err := c.GetServerTemplateDir(account, id, revision)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "server_template.yml"), nil
}

func (c *cache) PutServerTemplate(account int, id string, revision int, st *cm15.ServerTemplate) error {
	path, err := c.GetServerTemplateFile(account, id, revision)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	cst := CachedServerTemplate{st, "", fmt.Sprintf("%x", hash.Sum(nil))}
	item, err := os.Create(filepath.Join(filepath.Dir(path), "item.json"))
	encoder := json.NewEncoder(item)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(&cst); err != nil {
		return err
	}
	return nil
}

func (c *cache) GetRightScript(account int, id string, revision int) (rs CachedRightScript, ok bool, err error) {
	return
}

func (c *cache) GetRightScriptDir(account int, id string, revision int) (string, error) {
	return cacheCheck(c.rsPath, account, id, revision)
}

func (c *cache) GetRightScriptFile(account int, id string, revision int) (string, error) {
	dir, err := c.GetRightScriptDir(account, id, revision)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "right_script"), nil
}

func (c *cache) PutRightScript(account int, id string, revision int, rs *cm15.RightScript, attachments []string) error {
	path, err := c.GetRightScriptFile(account, id, revision)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	attachmentMD5s := make(map[string]string)
	for _, attachment := range attachments {
		file, err := os.Open(filepath.Join(filepath.Dir(path), "attachments", attachment))
		if err != nil {
			return err
		}
		defer file.Close()

		hash := md5.New()
		if _, err := io.Copy(hash, file); err != nil {
			return err
		}

		attachmentMD5s[attachment] = fmt.Sprintf("%x", hash.Sum(nil))
	}

	crs := CachedRightScript{rs, "", fmt.Sprintf("%x", hash.Sum(nil)), attachmentMD5s}
	item, err := os.Create(filepath.Join(filepath.Dir(path), "item.json"))
	encoder := json.NewEncoder(item)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(&crs); err != nil {
		return err
	}
	return nil
}

func (c *cache) GetMultiCloudImage(account int, id string, revision int) (mci CachedMultiCloudImage, ok bool, err error) {
	return
}

func (c *cache) GetMultiCloudImageDir(account int, id string, revision int) (string, error) {
	return cacheCheck(c.mciPath, account, id, revision)
}

func (c *cache) GetMultiCloudImageFile(account int, id string, revision int) (string, error) {
	dir, err := c.GetMultiCloudImageDir(account, id, revision)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "multi_cloud_image.yml"), nil
}

func (c *cache) PutMultiCloudImage(account int, id string, revision int, mci *cm15.MultiCloudImage) error {
	path, err := c.GetMultiCloudImageFile(account, id, revision)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	cmci := CachedMultiCloudImage{mci, "", fmt.Sprintf("%x", hash.Sum(nil))}
	item, err := os.Create(filepath.Join(filepath.Dir(path), "item.json"))
	encoder := json.NewEncoder(item)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(&cmci); err != nil {
		return err
	}
	return nil
}

func cacheCheck(path string, account int, id string, revision int) (string, error) {
	if revision == 0 {
		return "", errors.New("cannot cache HEAD revision")
	}

	path = filepath.Join(path, strconv.Itoa(account), id, fmt.Sprintf("%v", revision))
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", err
	}
	return path, nil
}
