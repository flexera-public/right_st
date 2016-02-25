package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"sort"
	"strconv"

	"github.com/go-yaml/yaml"
	// "github.com/inconshreveable/go-update"
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
	UpdateBaseUrl            = "https://binaries.rightscale.com/rsbin/right_st"
	UpdateGithubBaseUrl      = "https://github.com/rightscale/right_st"
	UpdateGithubReleasesUrl  = UpdateGithubBaseUrl + "/releases"
	UpdateGithubChangeLogUrl = UpdateGithubBaseUrl + "/blob/master/ChangeLog.md"
)

var (
	UpdateVersionUrl = UpdateBaseUrl + "/version.yml"

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

// UpdateGetLatestVersions gets the latest versions struct by downloading and parsing the version.yml file for right_st
// from the rsbin bucket. See version.sh and the Makefile upload target for how this file is created.
func UpdateGetLatestVersions() (*LatestVersions, error) {
	// get the version.yml file over HTTP(S)
	res, err := http.Get(UpdateVersionUrl)
	if err != nil {
		return nil, err
	}
	versions, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	// parse the version.yml file into a LatestVersions struct and return the result and any errors
	var latest LatestVersions
	err = yaml.Unmarshal(versions, &latest)
	return &latest, err
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

	updateAvailable := false

	// check if there is a new version for our major version and output a message if there is
	if latestForMajor, ok := latest.Versions[currentVersion.Major]; ok {
		if latestForMajor.GreaterThan(currentVersion) {
			fmt.Fprintf(output, "There is a new v%d version of %s (%s), to upgrade run:\n    %s update apply\n",
				latestForMajor.Major, app.Name, latestForMajor, app.Name)
			updateAvailable = true
		}
	}

	// check if there is a new major version and output a message if there is
	if latest.MajorVersion() > currentVersion.Major {
		fmt.Fprintf(output, "There is a new major version of %s (%s), to upgrade run:\n    %s update apply -m %d\n",
			app.Name, latest.Versions[latest.MajorVersion()], app.Name, latest.MajorVersion())
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
			fmt.Fprintf(output, "; you are using %s, to upgrade run:\n    %s update apply\n", currentVersion, app.Name)
		case latestGreater:
			fmt.Fprintf(output, "; you are using %s, to upgrade run:\n    %s update apply -m %d\n", currentVersion,
				app.Name, major)
		default:
			fmt.Fprintf(output, ".\n")
		}
	}
	fmt.Fprintf(output, "\nSee %s or\n%s for more information.\n", UpdateGithubChangeLogUrl, UpdateGithubReleasesUrl)

	return nil
}

// UpdateApply applies an update by downloading the tgz or zip file for the current platform, extracting it, and
// replacing the current executable with the new executable contained within. If majorVersion is 0, the tgz or zip for
// the latest update of the current major version will be used, otherwise the specified major version will be used.
func UpdateApply(vv string, output io.Writer, majorVersion uint) error {
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
