package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"runtime"
	"sort"
	"strconv"

	"github.com/go-yaml/yaml"
	"github.com/inconshreveable/go-update"
)

type Version struct {
	Major int
	Minor int
	Patch int
}

type LatestVersions struct {
	Versions     map[int]*Version
	majorVersion int
}

const (
	UpdateGithubBaseUrl      = "https://github.com/rightscale/right_st"
	UpdateGithubReleasesUrl  = UpdateGithubBaseUrl + "/releases"
	UpdateGithubChangeLogUrl = UpdateGithubBaseUrl + "/blob/master/ChangeLog.md"
)

var (
	UpdateBaseUrl = "https://binaries.rightscale.com/rsbin/right_st"

	vvString      = regexp.MustCompile(`^` + regexp.QuoteMeta(app.Name) + ` (v[0-9]+\.[0-9]+\.[0-9]+) -`)
	versionString = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)$`)
)

// UpdateGetCurrentVersion gets the current version struct from the version string defined in
// version.go/version_default.go.
func UpdateGetCurrentVersion(vv string) *Version {
	submatches := vvString.FindStringSubmatch(vv)
	// if the version string does not match it is a dev version and we cannot tell what actual version it is
	if submatches == nil {
		return nil
	}
	version, _ := NewVersion(submatches[1])
	return version
}

// UpdateGetVersionUrl gets the URL for the version.yml file for right_st from the rsbin bucket.
func UpdateGetVersionUrl() string {
	return UpdateBaseUrl + "/version-" + runtime.GOOS + "-" + runtime.GOARCH + ".yml"
}

// UpdateGetLatestVersions gets the latest versions struct by downloading and parsing the version.yml file for right_st
// from the rsbin bucket. See version.sh and the Makefile upload target for how this file is created.
func UpdateGetLatestVersions() (*LatestVersions, error) {
	// get the version.yml file over HTTP(S)
	res, err := http.Get(UpdateGetVersionUrl())
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Unexpected HTTP response getting %s: %s", UpdateGetVersionUrl(), res.Status)
	}
	versions, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	// parse the version.yml file into a LatestVersions struct and return the result and any errors
	var latest LatestVersions
	err = yaml.Unmarshal(versions, &latest)
	return &latest, err
}

// UpdateGetDownloadUrl gets the download URL of the latest version for a major version on the current operating system
// and architecture.
func UpdateGetDownloadUrl(majorVersion int) (string, *Version, error) {
	latest, err := UpdateGetLatestVersions()
	if err != nil {
		return "", nil, err
	}

	version, ok := latest.Versions[majorVersion]
	if !ok {
		return "", nil, fmt.Errorf("Major version not available: %d", majorVersion)
	}

	ext := "tgz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}

	return fmt.Sprintf("%s/%s/%s-%s-%s.%s", UpdateBaseUrl, version, app.Name, runtime.GOOS, runtime.GOARCH, ext),
		version, nil
}

// UpdateCheck checks if there is there are any updates available for the running current version of right_st and prints
// out instructions to upgrade if so. It will not perform a version check for dev versions (anything that has a version
// other than vX.Y.Z).
func UpdateCheck(vv string, output io.Writer) {
	currentVersion := UpdateGetCurrentVersion(vv)
	// do not check for updates on non release versions
	if currentVersion == nil {
		return
	}
	latest, err := UpdateGetLatestVersions()
	// we just ignore errors for update check
	if err != nil {
		return
	}

	// ignore more errors and continue since it will still return the right_st command name anyway
	sudoCommand, _ := updateSudoCommand()
	updateAvailable := false

	// check if there is a new version for our major version and output a message if there is
	if latestForMajor, ok := latest.Versions[currentVersion.Major]; ok {
		if latestForMajor.GreaterThan(currentVersion) {
			fmt.Fprintf(output, "There is a new v%d version of %s (%s), to upgrade run:\n    %s update apply\n",
				latestForMajor.Major, app.Name, latestForMajor, sudoCommand)
			updateAvailable = true
		}
	}

	// check if there is a new major version and output a message if there is
	if latest.MajorVersion() > currentVersion.Major {
		fmt.Fprintf(output, "There is a new major version of %s (%s), to upgrade run:\n    %s update apply -m %d\n",
			app.Name, latest.Versions[latest.MajorVersion()], sudoCommand, latest.MajorVersion())
		updateAvailable = true
	}

	// print informational URLs if there is any update available
	if updateAvailable {
		fmt.Fprintf(output, "\nSee %s or\n%s for more information.\n", UpdateGithubChangeLogUrl,
			UpdateGithubReleasesUrl)
	}
}

// UpdateList lists available versions based on the contents of the version.yml file for right_st in the rsbin bucket
// and prints upgrade instructions if any are applicable.
func UpdateList(vv string, output io.Writer) error {
	// get the current version from the version string, it will be nil if this is a dev version
	currentVersion := UpdateGetCurrentVersion(vv)
	latest, err := UpdateGetLatestVersions()
	if err != nil {
		return err
	}

	// sort the major versions so we can iterate through them in order
	majors := make([]int, 0, len(latest.Versions))
	for major, _ := range latest.Versions {
		majors = append(majors, int(major))
	}
	sort.Ints(majors)

	sudoCommand, err := updateSudoCommand()
	if err != nil {
		return err
	}

	// print out the latest version for each major version
	for _, major := range majors {
		version := latest.Versions[major]
		currentVersionEqual, latestForMajorGreater, latestGreater := false, false, false
		// don't check for updates for dev versions
		if currentVersion != nil {
			switch {
			case major == currentVersion.Major:
				currentVersionEqual = version.EqualTo(currentVersion)
				latestForMajorGreater = version.GreaterThan(currentVersion)
			case major == latest.MajorVersion():
				latestGreater = version.GreaterThan(currentVersion)
			}
		}

		fmt.Fprintf(output, "The latest v%d version of %s is %s", major, app.Name, version)
		switch {
		case currentVersionEqual:
			fmt.Fprintf(output, "; this is the version you are using!\n")
		case latestForMajorGreater:
			fmt.Fprintf(output, "; you are using %s, to upgrade run:\n    %s update apply\n", currentVersion,
				sudoCommand)
		case latestGreater:
			fmt.Fprintf(output, "; you are using %s, to upgrade run:\n    %s update apply -m %d\n", currentVersion,
				sudoCommand, major)
		default:
			fmt.Fprintf(output, ".\n")
		}
	}
	fmt.Fprintf(output, "\nSee %s or\n%s for more information.\n", UpdateGithubChangeLogUrl, UpdateGithubReleasesUrl)

	return nil
}

// UpdateApply applies an update by downloading the tgz or zip file for the current platform, extracting it, and
// replacing the current executable with the new executable contained within. If majorVersion is 0, the tgz or zip for
// the latest update of the current major version will be used, otherwise the specified major version will be used. If
// path is the empty string, the current executable will be replaced, otherwise it specifies the targetPath of the file
// to replace (this is only really used for testing).
func UpdateApply(vv string, output io.Writer, majorVersion int, targetPath string) error {
	// get the current version from the version string, it will be nil if this is a dev version
	currentVersion := UpdateGetCurrentVersion(vv)
	if majorVersion == 0 && currentVersion != nil {
		majorVersion = currentVersion.Major
	}

	// get the URL of the archive for the latest version for the major version
	url, version, err := UpdateGetDownloadUrl(majorVersion)
	if err != nil {
		return err
	}
	fmt.Fprintf(output, "Downloading %s %s from %s...\n", app.Name, version, url)

	// download the archive file from the URL
	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("Unexpected HTTP response getting %s: %s", url, res.Status)
	}

	// the new executable will need to be read from this reader which will come from somewhere inside the downloaded
	// archive
	var exe io.Reader
	exeName := app.Name
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}

	// get a reader for the new executable from the archive file which can be either a tgz (gzipped tar file) or a zip
	switch path.Ext(url) {
	case ".tgz":
		// create a gzip reader from the archive file stream in the HTTP response
		gzipReader, err := gzip.NewReader(res.Body)
		if err != nil {
			return err
		}
		defer gzipReader.Close()

		// create a tar reader from the gzip reader and iterate through its entries
		tarReader := tar.NewReader(gzipReader)
		for {
			// try to get the next header from the tar file
			header, err := tarReader.Next()
			if err == io.EOF {
				// if there was an EOF we've reached the end of the tar file
				break
			} else if err != nil {
				return err
			}

			// check if the current entry is for the new executable
			info := header.FileInfo()
			if !info.IsDir() && info.Name() == exeName {
				// assign the current entry's reader to be the new executable and stop iterating through the tar file
				exe = tarReader
				break
			}
		}
	case ".zip":
		// create a temporary file to store the zip archive file
		archive, err := ioutil.TempFile("", path.Base(url)+".")
		if err != nil {
			return err
		}
		defer func() {
			// on Windows you cannot delete a file that has open handles
			archive.Close()
			os.Remove(archive.Name())
		}()

		// write out the zip file to the temporary file from the HTTP response
		if _, err := io.Copy(archive, res.Body); err != nil {
			return err
		}
		if err := archive.Close(); err != nil {
			return err
		}

		// create a zip reader from the temporary file
		zipReader, err := zip.OpenReader(archive.Name())
		if err != nil {
			return err
		}
		defer zipReader.Close()

		// iterate through the files in the zip file
		for _, file := range zipReader.File {
			// check if the current file is for the new executable
			info := file.FileInfo()
			if !info.IsDir() && info.Name() == exeName {
				// open a reader for the current file in the zip file
				contents, err := file.Open()
				if err != nil {
					return err
				}
				defer contents.Close()

				// assign the current entry's reader to be the new executable and stop iterating through the zip file
				exe = contents
				break
			}
		}
	}

	// make sure we actually found the executable file in the archive
	if exe == nil {
		return fmt.Errorf("Could not find %s in archive: %s", exeName, url)
	}

	// attempt to apply the new executable so it replaces the old one
	if err := update.Apply(exe, update.Options{TargetPath: targetPath}); err != nil {
		// attempt to roll back if the apply failed
		if rollbackErr := update.RollbackError(err); rollbackErr != nil {
			return rollbackErr
		}
		return err
	}
	fmt.Fprintf(output, "Successfully updated %s to %s!\n", app.Name, version)

	return nil
}

// NewVersion creates a version struct from a version string of the form vX.Y.Z where X, Y, Z are the major, minor, and
// patch version numbers respectively. It returns a pointer to a version struct if sucessful or an error if there is a
// failure.
func NewVersion(version string) (*Version, error) {
	submatches := versionString.FindStringSubmatch(version)
	if submatches == nil {
		return nil, fmt.Errorf("Invalid version string: %s", version)
	}
	major, _ := strconv.ParseUint(submatches[1], 0, 0)
	minor, _ := strconv.ParseUint(submatches[2], 0, 0)
	patch, _ := strconv.ParseUint(submatches[3], 0, 0)
	return &Version{int(major), int(minor), int(patch)}, nil
}

// CompareTo compares a version struct to another version struct. It returns a value less than 0 if the version struct
// is less than the other, 0 if the version struct is equal to the other, or a value greater than 0 if the version
// struct is greater than the other.
func (v *Version) CompareTo(ov *Version) int {
	switch {
	case v.Major < ov.Major:
		return -1
	case v.Major > ov.Major:
		return +1
	default:
		switch {
		case v.Minor < ov.Minor:
			return -1
		case v.Minor > ov.Minor:
			return +1
		default:
			switch {
			case v.Patch < ov.Patch:
				return -1
			case v.Patch > ov.Patch:
				return +1
			}
		}
	}
	return 0
}

// EqualTo compares a version struct to another version struct in order to determine if the version struct is equal to
// the other or not.
func (v *Version) EqualTo(ov *Version) bool {
	return v.CompareTo(ov) == 0
}

// LessThan compares a version struct to another version struct in order to determine if the version struct is less than
// the other or not.
func (v *Version) LessThan(ov *Version) bool {
	return v.CompareTo(ov) < 0
}

// GreaterThan compares a version struct to another version struct in order to determine if the version struct is
// greater than the other or not.
func (v *Version) GreaterThan(ov *Version) bool {
	return v.CompareTo(ov) > 0
}

// String returns the string representation of a version struct which is vX.Y.Z.
func (v *Version) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// UnmarshalYAML parses the YAML representation of a version struct into a version struct. The YAML representation of a
// version struct is the same as the string representation.
func (v *Version) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	if err := unmarshal(&value); err != nil {
		return err
	}
	version, err := NewVersion(value)
	if err != nil {
		return err
	}
	*v = *version
	return nil
}

// MajorVersion gets the latest major version from a latest versions struct.
func (l *LatestVersions) MajorVersion() int {
	if l.majorVersion == 0 {
		for major, _ := range l.Versions {
			if major > l.majorVersion {
				l.majorVersion = major
			}
		}
	}
	return l.majorVersion
}
