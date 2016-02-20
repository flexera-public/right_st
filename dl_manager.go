// Copyright (c) 2014 RightScale, Inc. - see LICENSE

// Manager that downloads items cookbooks or attachments

package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
)

type downloadItem struct {
	url      url.URL
	filename string
	md5      string
}

// Limit concurrency of downloads
const maxConcurrentDownloads = 4

var downloadTokens = make(chan struct{}, maxConcurrentDownloads)

func init() {
	for i := 0; i < maxConcurrentDownloads; i++ {
		downloadTokens <- struct{}{}
	}
}

// downloadManager download a bunch of "items", typically attachments or cookbooks
func downloadManager(items []*downloadItem) error {
	var err error
	var size int64
	var lock sync.Mutex // for both size and err

	setError := func(e error) {
		defer lock.Unlock()
		lock.Lock()
		err = e
	}
	hasError := func() bool {
		defer lock.Unlock()
		lock.Lock()
		return err != nil
	}
	sumSize := func(sz int64) {
		defer lock.Unlock()
		lock.Lock()
		size = sz
	}

	t := time.Now()

	// download all attachments with some parallelism
	var wg sync.WaitGroup
	for _, i := range items {
		wg.Add(1)
		go func(i *downloadItem) {
			defer wg.Done()
			var e error
			for try := uint32(0); try < 10; try++ {
				// wait for download slot
				<-downloadTokens
				defer func() {
					downloadTokens <- struct{}{}
				}()
				// only download if there's not an error already
				if hasError() {
					return
				}
				var retry bool
				var size int64
				retry, size, e = downloadOneItem(i)
				if e == nil {
					sumSize(size)
					return
				}
				// fmt.Printf("        Error downloading %s:", path.Base(i.filename))
				// fmt.Printf("          %s", e.Error())
				if !retry {
					setError(e)
					return
				}
				time.Sleep((1 << try) * time.Second / 2)
			}
			setError(e)
		}(i)
	}
	wg.Wait()
	dt := time.Since(t)
	fmt.Printf("    Done with %d attachments: %dKB in %.1fs -> %.3fMB/s\n", len(items),
		size/1024, float32(dt)/float32(time.Second),
		float32(size)/1024/1024/(float32(dt)/float32(time.Second)))
	return err
}

// downloadItem downloads an individual item into the cache directory. It returns
// a boolean that is true if an error occurred that is retryable; also returns an int, which
// is the size of the item.
func downloadOneItem(item *downloadItem) (bool, int64, error) {
	// Check existence
	if _, err := os.Stat(item.filename); err == nil {
		// file already exists, don't re-download
		existingMd5sum, err := fmd5sum(item.filename)
		if err != nil {
			return false, 0, err
		}
		if item.md5 == existingMd5sum {
			fmt.Printf("    Skipping attachments/%s, already downloaded\n", path.Base(item.filename))
			return false, 0, nil
		}
	}
	// Create parent directory
	err := os.MkdirAll(filepath.Dir(item.filename), 0755)
	if err != nil {
		return false, 0, fmt.Errorf("Erroring creating directory: %s", err.Error())
	}

	// Open the file
	f, err := os.Create(item.filename)
	if err != nil {
		return false, 0, fmt.Errorf("Error creating: %s", err.Error())
	}
	defer f.Close()

	// Do the download
	startAt := time.Now()
	fmt.Printf("    Downloading attachment into attachments/%s\n", path.Base(item.filename))
	resp, err := http.Get(item.url.String())
	if err != nil {
		f.Close() // on Windows you cannot remove a file that has an open file handle
		os.Remove(item.filename)
		retry := false
		if netErr, ok := err.(net.Error); ok {
			retry = netErr.Timeout() || netErr.Temporary()
		}
		return retry, 0, fmt.Errorf("%s -- URL=%s", err.Error(), item.url.String())
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		f.Close() // on Windows you cannot remove a file that has an open file handle
		os.Remove(item.filename)
		return resp.StatusCode >= 500, 0, fmt.Errorf("%s -- URL=%s", resp.Status, item.url.String())
	}
	// read the attachment body
	size, err := io.Copy(f, resp.Body)
	if err != nil {
		f.Close() // on Windows you cannot remove a file that has an open file handle
		os.Remove(item.filename)
		return true, 0, fmt.Errorf("%s -- Reading %s", err.Error(), path.Base(item.filename))
	}
	if *debug {
		fmt.Printf("    %.1fKB in %.1fs for %s", float32(size)/1024,
			time.Since(startAt).Seconds(), path.Base(item.filename))
	}
	return false, size, nil
}
