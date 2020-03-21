package main

import (
	"path/filepath"

	"github.com/douglaswth/rsrdp/win32"
)

func DefaultConfigFile() string {
	roamingPath, err := win32.SHGetKnownFolderPath(&win32.FOLDERID_RoamingAppData, 0, 0)
	if err != nil {
		panic(err)
	}

	return filepath.Join(roamingPath, "RightST", ".right_st.yml")
}

func DefaultCachePath() string {
	roamingPath, err := win32.SHGetKnownFolderPath(&win32.FOLDERID_RoamingAppData, 0, 0)
	if err != nil {
		panic(err)
	}

	return filepath.Join(roamingPath, "RightST", ".right_st.cache")
}
