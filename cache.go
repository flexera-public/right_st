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
		GetServerTemplate(account int, id string, revision int) (*CachedServerTemplate, error)
		GetServerTemplateDir(account int, id string, revision int) (path string, err error)
		GetServerTemplateFile(account int, id string, revision int) (path string, err error)
		PutServerTemplate(account int, id string, revision int, st *cm15.ServerTemplate) error

		GetRightScript(account int, id string, revison int) (*CachedRightScript, error)
		GetRightScriptDir(account int, id string, revision int) (path string, err error)
		GetRightScriptFile(account int, id string, revision int) (path string, err error)
		PutRightScript(account int, id string, revision int, rs *cm15.RightScript, attachments []string) error

		GetMultiCloudImage(account int, id string, revision int) (*CachedMultiCloudImage, error)
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

func (c *cache) GetServerTemplate(account int, id string, revision int) (*CachedServerTemplate, error) {
	file, err := c.GetServerTemplateFile(account, id, revision)
	if err != nil {
		return nil, err
	}

	item, err := os.Open(filepath.Join(filepath.Dir(file), "item.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer item.Close()

	cst := &CachedServerTemplate{File: file}
	decoder := json.NewDecoder(item)
	if err := decoder.Decode(cst); err != nil {
		return nil, err
	}

	ok, err := cacheCheckMD5(file, cst.MD5Sum)
	if err != nil {
		return nil, err
	}
	if !ok {
		item.Close()

		err := os.RemoveAll(filepath.Dir(file))
		if err != nil {
			return nil, err
		}
		return nil, nil
	}

	return cst, nil
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
	sum, err := cacheMD5(path)
	if err != nil {
		return err
	}

	cst := CachedServerTemplate{st, "", sum}

	item, err := os.Create(filepath.Join(filepath.Dir(path), "item.json"))
	if err != nil {
		return err
	}
	defer item.Close()

	encoder := json.NewEncoder(item)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(&cst); err != nil {
		return err
	}
	return nil
}

func (c *cache) GetRightScript(account int, id string, revision int) (*CachedRightScript, error) {
	file, err := c.GetRightScriptFile(account, id, revision)
	if err != nil {
		return nil, err
	}

	item, err := os.Open(filepath.Join(filepath.Dir(file), "item.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer item.Close()

	crs := &CachedRightScript{File: file}
	decoder := json.NewDecoder(item)
	if err := decoder.Decode(crs); err != nil {
		return nil, err
	}

	ok, err := cacheCheckMD5(file, crs.MD5Sum)
	if err != nil {
		return nil, err
	}
	if ok {
		attachmentsDir := filepath.Join(filepath.Dir(file), "attachments")
		for attachment, md5Sum := range crs.Attachments {
			ok, err = cacheCheckMD5(filepath.Join(attachmentsDir, attachment), md5Sum)
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}
		}
	}
	if !ok {
		item.Close()

		err := os.RemoveAll(filepath.Dir(file))
		if err != nil {
			return nil, err
		}
		return nil, nil
	}

	return crs, nil
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
	sum, err := cacheMD5(path)
	if err != nil {
		return err
	}

	attachmentsDir := filepath.Join(filepath.Dir(path), "attachments")
	attachmentMD5s := make(map[string]string)
	for _, attachment := range attachments {
		sum, err := cacheMD5(filepath.Join(attachmentsDir, attachment))
		if err != nil {
			return err
		}
		attachmentMD5s[attachment] = sum
	}

	crs := CachedRightScript{rs, "", sum, attachmentMD5s}

	item, err := os.Create(filepath.Join(filepath.Dir(path), "item.json"))
	if err != nil {
		return err
	}
	defer item.Close()

	encoder := json.NewEncoder(item)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(&crs); err != nil {
		return err
	}
	return nil
}

func (c *cache) GetMultiCloudImage(account int, id string, revision int) (*CachedMultiCloudImage, error) {
	file, err := c.GetMultiCloudImageFile(account, id, revision)
	if err != nil {
		return nil, err
	}

	item, err := os.Open(filepath.Join(filepath.Dir(file), "item.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer item.Close()

	cmci := &CachedMultiCloudImage{File: file}
	decoder := json.NewDecoder(item)
	if err := decoder.Decode(cmci); err != nil {
		return nil, err
	}

	ok, err := cacheCheckMD5(file, cmci.MD5Sum)
	if err != nil {
		return nil, err
	}
	if !ok {
		item.Close()

		err := os.RemoveAll(filepath.Dir(file))
		if err != nil {
			return nil, err
		}
		return nil, nil
	}

	return cmci, nil
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
	sum, err := cacheMD5(path)
	if err != nil {
		return err
	}

	cmci := CachedMultiCloudImage{mci, "", sum}

	item, err := os.Create(filepath.Join(filepath.Dir(path), "item.json"))
	if err != nil {
		return err
	}
	defer item.Close()

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

func cacheCheckMD5(path, md5Sum string) (bool, error) {
	sum, err := cacheMD5(path)
	if err != nil {
		return false, err
	}
	return sum == md5Sum, nil
}

func cacheMD5(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
