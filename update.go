package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"

	"github.com/go-yaml/yaml"
)

type Version struct {
	Major uint
	Minor uint
	Patch uint
}

type LatestVersions struct {
	Versions     map[uint]*Version
	majorVersion uint
}

const UpdateBaseUrl = "https://binaries.rightscale.com/rsbin/right_st/"

var (
	UpdateVersionUrl = UpdateBaseUrl + "version.yml"

	vvString      = regexp.MustCompile(`^` + regexp.QuoteMeta(app.Name) + ` (v[0-9]+\.[0-9]+\.[0-9]+) -`)
	versionString = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)$`)
)

func UpdateGetCurrentVersion(vv string) *Version {
	submatches := vvString.FindStringSubmatch(vv)
	if submatches == nil {
		return nil
	}
	version, _ := NewVersion(submatches[1])
	return version
}

func UpdateGetLatestVersions() (*LatestVersions, error) {
	res, err := http.Get(UpdateVersionUrl)
	if err != nil {
		return nil, err
	}
	versions, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	var latest LatestVersions
	err = yaml.Unmarshal(versions, &latest)
	return &latest, err
}

func UpdateCheck(vv string, output io.Writer) {
	currentVersion := UpdateGetCurrentVersion(vv)
	// do not check for updates on non release versions
	if currentVersion == nil {
		return
	}
	latest, err := UpdateGetLatestVersions()
	if err != nil {
		return
	}
	if latestForMajor, ok := latest.Versions[currentVersion.Major]; ok {
		if latestForMajor.GreaterThan(currentVersion) {
			fmt.Fprintf(output, "There is a new v%d version of %s (%s), to upgrade run:\n    %s update apply\n",
				latestForMajor.Major, app.Name, latestForMajor, app.Name)
		}
	}
	if latest.MajorVersion() > currentVersion.Major {
		fmt.Fprintf(output, "There is a new major version of %s (%s), to upgrade run:\n    %s update apply -m %d\n",
			app.Name, latest.Versions[latest.MajorVersion()], app.Name, latest.MajorVersion())
	}
}

func UpdateList(vv string, output io.Writer) error {
	return nil
}

func UpdateApply(vv string, output io.Writer, majorVersion uint) error {
	return nil
}

func NewVersion(version string) (*Version, error) {
	submatches := versionString.FindStringSubmatch(version)
	if submatches == nil {
		return nil, fmt.Errorf("Invalid version string: %s", version)
	}
	major, _ := strconv.ParseUint(submatches[1], 0, 0)
	minor, _ := strconv.ParseUint(submatches[2], 0, 0)
	patch, _ := strconv.ParseUint(submatches[3], 0, 0)
	return &Version{uint(major), uint(minor), uint(patch)}, nil
}

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

func (v *Version) EqualTo(ov *Version) bool {
	return v.CompareTo(ov) == 0
}

func (v *Version) LessThan(ov *Version) bool {
	return v.CompareTo(ov) < 0
}

func (v *Version) GreaterThan(ov *Version) bool {
	return v.CompareTo(ov) > 0
}

func (v *Version) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

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

func (l *LatestVersions) MajorVersion() uint {
	if l.majorVersion == 0 {
		for major, _ := range l.Versions {
			if major > l.majorVersion {
				l.majorVersion = major
			}
		}
	}
	return l.majorVersion
}
