package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

	"github.com/go-yaml/yaml"
	"github.com/rightscale/rsc/rsapi"
)

type Image struct {
	Cloud     string `yaml:"Cloud"`
	Id        string `yaml:"Id"`
	Name      string `yaml:"Name"`
	cloudHref string
}

type ServerTemplate struct {
	Name             string                    `yaml:"Name"`
	Description      string                    `yaml:"Description"`
	Inputs           map[string]*InputMetadata `yaml:"Inputs"`
	MultiCloudImages []string                  `yaml:"MultiCloudImages"`
	Images           []Image                   `yaml:"Images"`
	RightScripts     []string                  `yaml:"RightScripts"`
	mciHrefs         []string
}

func stUpload(files []string) {
	fmt.Printf("%v", files)
	for _, file := range files {

		st, errors := validateServerTemplate(file)
		if len(errors) != 0 {
			fmt.Println("Encountered the following errors with the ServerTemplate:")
			for _, err := range errors {
				fmt.Println(err)
			}
			os.Exit(1)
		}

		fmt.Printf("%#v", *st)
	}
	// Read and validate .yml describing ST, including validation of all RightScripts
	// contained within.

	// Upload and create ST
}

func stShow(href string) {
	client := config.environment.Client15()

	stLocator := client.ServerTemplateLocator(href)

	st, err := stLocator.Show(rsapi.APIParams{"view": "inputs_2_0"})
	if err != nil {
		fatalError("Could not find ServerTemplate with href %s: %s", href, err.Error())
	}

	mciLocator := client.MultiCloudImageLocator(getLink(st.Links, "multi_cloud_images"))
	mcis, err := mciLocator.Index(rsapi.APIParams{})
	if err != nil {
		fatalError("Could not find MCIs with href %s: %s", mciLocator.Href, err.Error())
	}

	rbLocator := client.RunnableBindingLocator(getLink(st.Links, "runnable_bindings"))
	rbs, err := rbLocator.Index(rsapi.APIParams{})
	if err != nil {
		fatalError("Could not find attached RightScripts with href %s: %s", rbLocator.Href, err.Error())
	}

	rev := "HEAD"
	if st.Revision != 0 {
		rev = fmt.Sprintf("%d", st.Revision)
	}
	stHref := getLink(st.Links, "self")

	fmt.Printf("Name: %s\n", st.Name)
	fmt.Printf("HREF: %s\n", stHref)
	fmt.Printf("Revision: %s\n", rev)
	fmt.Printf("Description: \n%s\n", st.Description)
	fmt.Printf("MultiCloudImages: (href, rev, name) \n")
	for _, item := range mcis {
		mciHref := getLink(item.Links, "self")
		rev := "HEAD"
		if item.Revision != 0 {
			rev = fmt.Sprintf("%d", item.Revision)
		}
		fmt.Printf("  %s %5s %s\n", mciHref, rev, item.Name)
	}
	sequenceTypes := []string{"boot", "operational", "decommission"}
	for _, sequenceType := range sequenceTypes {
		fmt.Printf("RightScripts - %s: (href, rev, name)\n", sequenceType)
		for _, item := range rbs {
			rsHref := getLink(item.Links, "right_script")
			//if item.RightScript != cm15.RightScript(nil) {
			rs := item.RightScript
			if item.Sequence != sequenceType {
				continue
			}
			rev := "HEAD"
			if rs.Revision != 0 {
				rev = fmt.Sprintf("%d", rs.Revision)
			}
			fmt.Printf("  %s %5s %s\n", rsHref, rev, rs.Name)
			// } else {
			//  fmt.Printf(" RECIPE - NOT HANDLED YET")
			// }
		}
	}
}

func validateServerTemplate(file string) (*ServerTemplate, []error) {
	st := ServerTemplate{}
	var errors []error

	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		errors = append(errors, err)
		return nil, errors
	}
	err = yaml.Unmarshal(bytes, &st)
	if err != nil {
		errors = append(errors, err)
		return nil, errors
	}
	client := config.environment.Client15()

	idMatch := regexp.MustCompile(`^\d+$`)
	hrefMatch := regexp.MustCompile(`^/api/multi_cloud_images/\d+$`)
	st.mciHrefs = make([]string, len(st.MultiCloudImages))
	for i, mciIdOrHref := range st.MultiCloudImages {
		// TBD let people specify names. This gets tricky with multiple revisions and
		// naming conflicts though so just have them use hrefs for now.
		var href string
		if idMatch.Match([]byte(mciIdOrHref)) {
			href = fmt.Sprintf("/api/multi_cloud_images/%s", mciIdOrHref)
		} else if hrefMatch.Match([]byte(mciIdOrHref)) {
			href = mciIdOrHref
		} else {
			errors = append(errors, fmt.Errorf("MultiCloudImage parameter %s is not a MCI HREF", mciIdOrHref))
		}
		loc := client.MultiCloudImageLocator(href)
		_, err := loc.Show()
		if err != nil {
			errors = append(errors, fmt.Errorf("Could not find MCI HREF %s in account", href))
		}
		st.mciHrefs[i] = href
	}

	for i, img := range st.Images {
		if img.Name == "" || img.Cloud == "" || img.Id == "" {
			errors = append(errors, fmt.Errorf("Image parameter %d must have Name, Cloud, and Id specified", i))
		}

	}

	for _, rsName := range st.RightScripts {
		_, err := validateRightScript(rsName)
		if err != nil {
			rsError := fmt.Errorf("RightScript error: %s: %s", rsName, err.Error())
			errors = append(errors, rsError)
		}
	}

	return &st, errors
}
