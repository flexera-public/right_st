package main

import (
	"io"
	"io/ioutil"
	"time"

	"github.com/pmezard/go-difflib/difflib"
)

func Diff(nameA, nameB string, timeA, timeB time.Time, rA, rB io.Reader) (bool, string, error) {
	bA, err := ioutil.ReadAll(rA)
	if err != nil {
		return false, "", err
	}
	bB, err := ioutil.ReadAll(rB)
	if err != nil {
		return false, "", err
	}

	// TODO deal with binary stuff; probably using http.DetectContentType

	ud := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(bA)),
		B:        difflib.SplitLines(string(bB)),
		FromFile: nameA,
		ToFile:   nameB,
		Context:  3,
	}

	if !timeA.IsZero() {
		ud.FromDate = timeA.Format(time.RFC3339)
	}
	if !timeB.IsZero() {
		ud.ToDate = timeA.Format(time.RFC3339)
	}

	d, err := difflib.GetUnifiedDiffString(ud)
	if err != nil {
		return false, "", err
	}

	return d != "", d, nil
}
