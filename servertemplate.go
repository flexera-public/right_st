package main

import (
	"fmt"

	"github.com/rightscale/rsc/rsapi"
)

func stUpload(files []string) {

	fmt.Printf("%v", files)
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
