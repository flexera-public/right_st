package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/pmezard/go-difflib/difflib"
)

// Diff returns true if two files differ and a unified diff string
// (or a message that the files differ if either is binary)
func Diff(nameA, nameB string, timeA, timeB time.Time, rA, rB io.Reader) (bool, string, error) {
	bA, err := ioutil.ReadAll(rA)
	if err != nil {
		return false, "", err
	}
	bB, err := ioutil.ReadAll(rB)
	if err != nil {
		return false, "", err
	}

	// this should be good enough for determining if files are text or binary
	typeA := http.DetectContentType(bA)
	typeB := http.DetectContentType(bB)

	if *debug {
		fmt.Printf("%v type: %v\n%v type: %v\n", nameA, typeA, nameB, typeB)
	}

	// create a unified diff if both files are text
	if strings.HasPrefix(typeA, "text/") && strings.HasPrefix(typeB, "text/") {
		ud := difflib.UnifiedDiff{
			A:        difflib.SplitLines(string(bA)),
			B:        difflib.SplitLines(string(bB)),
			FromFile: nameA,
			ToFile:   nameB,
			Context:  3,
		}

		// include non-zero timestamps in the unified diff
		if !timeA.IsZero() {
			ud.FromDate = timeA.Format(time.RFC3339)
		}
		if !timeB.IsZero() {
			ud.ToDate = timeB.Format(time.RFC3339)
		}

		d, err := difflib.GetUnifiedDiffString(ud)
		if err != nil {
			return false, "", err
		}

		// work around difflib including an extra " \n" at the end of unified diffs
		return d != "", strings.TrimSuffix(d, " \n"), nil
	}

	// check if binary files differ
	if bytes.Equal(bA, bB) {
		return false, "", nil
	}

	return true, fmt.Sprintf("Binary files %v and %v differ\n", nameA, nameB), nil
}
