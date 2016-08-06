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
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type downloadItem struct {
	url url.URL
	// try and download the file to the following locations. we allow multiple locations in case the first one is taken
	locations    []string
	md5          string
	downloadedTo string // which of the locations did we use when we downloaded the file?
	size         int64  // size if bytes that were downloaded
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
		size += sz
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
				retry, e = downloadOneItem(i)
				if e == nil {
					sumSize(i.size)
					return
				}
				// fmt.Printf("        Error downloading %s:", filepath.Base(i.filename))
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

// downloadItem downloads an individual item to disk. It returns a boolean that
// is true if an error occurred that is retryable
func downloadOneItem(item *downloadItem) (bool, error) {
	effectiveName := ""
	for _, filename := range item.locations {
		md5sum, err := fmd5sum(filename)
		if err == nil {
			// File already exists. If the md5sum matches, we're golden. else do nothing and try the next location
			if item.md5 == md5sum {
				fmt.Printf("    Skipping attachment '%s', already downloaded\n", filepath.Base(filename))
				item.downloadedTo = filename
				return false, nil
			}
		} else {
			// File doesn't exist, so we can use this filename
			effectiveName = filename
			break
		}
	}

	if effectiveName == "" {
		return false, fmt.Errorf("File '%s' already exists with a different md5sum at locations [%s]",
			filepath.Base(item.locations[0]), strings.Join(item.locations, ", "))
	}

	// Create parent directory
	err := os.MkdirAll(filepath.Dir(effectiveName), 0755)
	if err != nil {
		return false, fmt.Errorf("Erroring creating directory: %s", err.Error())
	}

	// Open the file
	f, err := os.Create(effectiveName)
	if err != nil {
		return false, fmt.Errorf("Error creating: %s", err.Error())
	}
	defer f.Close()

	// Do the download
	startAt := time.Now()
	fmt.Printf("    Downloading attachment '%s' to '%s'\n", filepath.Base(effectiveName), effectiveName)
	resp, err := http.Get(item.url.String())
	if err != nil {
		f.Close() // on Windows you cannot remove a file that has an open file handle
		os.Remove(effectiveName)
		retry := false
		if netErr, ok := err.(net.Error); ok {
			retry = netErr.Timeout() || netErr.Temporary()
		}
		return retry, fmt.Errorf("%s -- URL=%s", err.Error(), item.url.String())
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		f.Close() // on Windows you cannot remove a file that has an open file handle
		os.Remove(effectiveName)
		return resp.StatusCode >= 500, fmt.Errorf("%s -- URL=%s", resp.Status, item.url.String())
	}
	// read the attachment body
	size, err := io.Copy(f, resp.Body)
	if err != nil {
		f.Close() // on Windows you cannot remove a file that has an open file handle
		os.Remove(effectiveName)
		return true, fmt.Errorf("%s -- Reading %s", err.Error(), filepath.Base(effectiveName))
	}
	if *debug {
		fmt.Printf("    %.1fKB in %.1fs for %s", float32(size)/1024,
			time.Since(startAt).Seconds(), filepath.Base(effectiveName))
	}
	item.size = size
	item.downloadedTo = effectiveName
	return false, nil
}
