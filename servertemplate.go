package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/go-yaml/yaml"
	"github.com/rightscale/rsc/cm15"
	"github.com/rightscale/rsc/rsapi"
)

type Image struct {
	Href     string `yaml:"Href"`
	Cloud    string `yaml:"Cloud"`
	Image    string `yaml:"Image"`
	Name     string `yaml:"Name"`
	Revision int    `yaml:"Revision"`
}

type ServerTemplate struct {
	Name             string                    `yaml:"Name"`
	Description      string                    `yaml:"Description"`
	Inputs           map[string]*InputMetadata `yaml:"Inputs"`
	MultiCloudImages []*Image                  `yaml:"MultiCloudImages"`
	RightScripts     map[string][]string       `yaml:"RightScripts"`
	mciHrefs         []string
}

var sequenceTypes []string = []string{"Boot", "Operational", "Decommission"}

func stUpload(files []string) {

	for _, file := range files {
		fmt.Printf("Uploading %s\n", file)
		st, rightscripts, errors := validateServerTemplate(file)
		if len(errors) != 0 {
			fmt.Println("Encountered the following errors with the ServerTemplate:")
			for _, err := range errors {
				fmt.Println(err)
			}
			os.Exit(1)
		}

		fmt.Printf("ST: %#v\n", *st)
		err := doUpload(*st, *rightscripts)

		if err != nil {
			fatalError("Failed to upload ServerTemplate '%s': %s", file, err.Error())
		}
	}
}

// Options:
//   -- commit
func doUpload(stDef ServerTemplate, rightScriptsDef map[string][]RightScript) error {
	// Check if ST with same name (HEAD revisions only) exists. If it does, update the head
	client := config.environment.Client15()
	st, err := getServerTemplateByName(stDef.Name)

	if err != nil {
		fatalError("Failed to query for ServerTemplate '%s': %s", stDef.Name, err.Error())
	}

	// -----------------
	// Synchronize ST
	// -----------------
	// st = ST cloud object. stDef = ST defined in YAML on disk
	stVerb := "Updating"
	if st == nil {
		params := cm15.ServerTemplateParam{
			Description: stDef.Description,
			Name:        stDef.Name,
		}
		stLoc, err := client.ServerTemplateLocator("/api/server_templates").Create(&params)
		if err != nil {
			fatalError("Failed to create ServerTemplate '%s': %s", stDef.Name, err.Error())
		}
		st, err = stLoc.Show(rsapi.APIParams{})
		if err != nil {
			fatalError("Failed to refetch ServerTemplate '%s': %s", stLoc.Href, err.Error())
		}
		stVerb = "Creating"
	}
	stHref := getLink(st.Links, "self")
	fmt.Printf("%s ServerTemplate %s\n", stVerb, getLink(st.Links, "self"))

	// -----------------
	// Synchonize MCIs
	// -----------------
	// Get a list of MCIs on the existing ST.
	mciLocator := client.ServerTemplateMultiCloudImageLocator("/api/server_template_multi_cloud_images")

	existingMcis, err := mciLocator.Index(rsapi.APIParams{"filter": []string{"server_template_href==" + stHref}})
	if err != nil {
		fatalError("Could not find MCIs with href %s: %s", mciLocator.Href, err.Error())
	}
	// Delete MCIs that exist on the existing ST but not in this definition.
	for _, mci := range existingMcis {
		mciHref := getLink(mci.Links, "multi_cloud_image")
		foundMci := false // found on ST definition
		for _, stDefHref := range stDef.mciHrefs {
			if stDefHref == mciHref {
				foundMci = true
			}
		}
		if !foundMci {
			fmt.Printf("Removing MCI %s\n", mciHref)
			mci.Locator(client).Destroy()
		}
	}

	// Add all MCIs. If the MCI has not changed (HREF is the same, or combination of values is the same) don't update?
	for i, stDefMciHref := range stDef.mciHrefs {
		foundMci := false // found on ST
		for _, mci := range existingMcis {
			mciHref := getLink(mci.Links, "multi_cloud_image")
			if stDefMciHref == mciHref {
				foundMci = true
			}
		}
		if !foundMci {
			params := cm15.ServerTemplateMultiCloudImageParam{
				MultiCloudImageHref: stDefMciHref,
				ServerTemplateHref:  stHref,
			}
			fmt.Printf("Adding MCI %s\n", stDefMciHref)
			loc, err := mciLocator.Create(&params)
			if err != nil {
				fatalError("Failed to associate MCI '%s' with ServerTemplate '%s': %s", stDefMciHref, stHref, err.Error())
			}
			if i == 0 {
				_ = loc.MakeDefault()
			}
		}
	}

	// -----------------
	// Synchronize RightScripts
	// -----------------

	// By the time we get to here, we've done a good bit of error checking, so don't have to recheck much.
	// We know the files on disk are all openable, the RightScripts on disk have valid metadata.
	//   TBD: issue: we don't check for duplicate rightscript names till .Push(). move that check to validation section?
	// Get RightScript object in RightScale. RightScript.Push() handles both cases below:
	//		1. Doesn't exist: create
	//		2. Exists: Update contents
	for _, sequenceType := range sequenceTypes {
		for _, script := range rightScriptsDef[sequenceType] {
			// Push() has the side effort of always populating script.Href which we use below -- probably
			// rework this to be a bit more upfront in the future.
			err := script.Push()
			if err != nil {
				fatalError("%s", err.Error())
			}
		}
	}

	// Add new RightScripts to the sequence list. Don't worry about order for now, that'll be fixed up below
	rbLoc := client.RunnableBindingLocator(getLink(st.Links, "runnable_bindings"))
	existingRbs, _ := rbLoc.Index(rsapi.APIParams{})
	seenExistingRbs := make([]bool, len(existingRbs), len(existingRbs))
	for _, sequenceType := range sequenceTypes {
		for _, script := range rightScriptsDef[sequenceType] {
			seenScript := false
			for i, rb := range existingRbs {
				rbHref := getLink(rb.Links, "right_script")
				if rb.Sequence == strings.ToLower(sequenceType) && rbHref == script.Href {
					seenScript = true
					seenExistingRbs[i] = true
				}
			}
			if !seenScript {
				params := cm15.RunnableBindingParam{
					RightScriptHref: script.Href,
					Sequence:        strings.ToLower(sequenceType),
				}
				fmt.Printf("Adding %s to ServerTemplate\n", script.Href)
				_, err := rbLoc.Create(&params)
				if err != nil {
					fatalError("Could not create %s RunnableBinding for HREF %s: %s", sequenceType, script.Href, err.Error())
				}
			}
		}
	}
	// Remove RightScripts that don't belong from the sequence list
	for i, rb := range existingRbs {
		if !seenExistingRbs[i] {
			fmt.Printf("Removing %s from ServerTemplate\n", getLink(rb.Links, "right_script"))
			err := rb.Locator(client).Destroy()
			if err != nil {
				fatalError("Could not destroy RunnableBinding %s: %s", getLink(rb.Links, "right_script"), err.Error())
			}
		}
	}

	// All RightScripts should now be attached. Do a multi_update to get the position numbers correct
	existingRbs, _ = rbLoc.Index(rsapi.APIParams{})
	rbLookup := make(map[string]*cm15.RunnableBinding)
	for _, rb := range existingRbs {
		key := rb.Sequence + "_" + getLink(rb.Links, "right_script")
		rbLookup[key] = rb
	}

	for _, sequenceType := range sequenceTypes {
		bindings := []*cm15.RunnableBindings{}
		for i, script := range rightScriptsDef[sequenceType] {
			key := strings.ToLower(sequenceType) + "_" + script.Href
			rb, ok := rbLookup[key]
			if !ok {
				fatalError("Could not lookup RunnableBinding %s", key)
			}
			b := cm15.RunnableBindings{
				Id:       rb.Id,
				Position: fmt.Sprintf("%d", i+1),
				Sequence: rb.Sequence,
			}
			bindings = append(bindings, &b)
		}
		err := rbLoc.MultiUpdate(bindings)
		if err != nil {
			fatalError("MultiUpdatet to set RunnableBinding order for %s failed: %s", sequenceType, err.Error())
		}
	}

	// If the user requested a commit on changes, commit the ST. This will commit all RightScripts as well.
	return nil
}

// TBD
//   Show uncommitted changes
//   Show a list of previous revisions?
//   If we're not head, show a link to the head revision/lineage?
//   AlertSpecs
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
	for _, sequenceType := range sequenceTypes {
		fmt.Printf("RightScripts - %s: (href, rev, name)\n", sequenceType)
		for _, item := range rbs {
			rsHref := getLink(item.Links, "right_script")
			//if item.RightScript != cm15.RightScript(nil) {
			rs := item.RightScript
			if item.Sequence != strings.ToLower(sequenceType) {
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

// TBD
//   AlertSpecs
//   Cookbooks in some way?
func validateServerTemplate(file string) (*ServerTemplate, *map[string][]RightScript, []error) {
	var errors []error

	f, err := os.Open(file)
	if err != nil {
		errors = append(errors, err)
		return nil, nil, errors
	}
	defer f.Close()

	st, err := ParseServerTemplate(f)
	if err != nil {
		errors = append(errors, err)
		return nil, nil, errors
	}

	client := config.environment.Client15()

	//idMatch := regexp.MustCompile(`^\d+$`)
	hrefMatch := regexp.MustCompile(`^/api/multi_cloud_images/\d+$`)
	st.mciHrefs = make([]string, len(st.MultiCloudImages))
	// Let people specify MCIs multiple ways:
	//   1. Href
	//   2. Name/revision pair (TBD)
	//   3. Name/Cloud/Image combination (TBD)
	for i, image := range st.MultiCloudImages {
		var href string
		if hrefMatch.Match([]byte(image.Href)) {
			href = image.Href
		} else if image.Name != "" && image.Revision != 0 {
			errors = append(errors, fmt.Errorf("Cannot parse MCIs by name/revision yet"))
			continue
		} else if image.Name != "" && image.Cloud != "" && image.Image != "" {
			errors = append(errors, fmt.Errorf("Cannot parse MCIs by cloud/image yet"))
			continue
		} else {
			errors = append(errors, fmt.Errorf("MultiCloudImage item must be a hash with 'Href' key set to a valid value"))
			continue
		}
		loc := client.MultiCloudImageLocator(href)
		_, err := loc.Show()
		if err != nil {
			errors = append(errors, fmt.Errorf("Could not find MCI HREF %s in account", href))
		}
		st.mciHrefs[i] = href
	}

	rightscripts := make(map[string][]RightScript)
	for sequence, scripts := range st.RightScripts {
		for _, rsName := range scripts {
			rs, err := validateRightScript(rsName, false)
			if err != nil {
				rsError := fmt.Errorf("RightScript error: %s:%s: %s", sequence, rsName, err.Error())
				errors = append(errors, rsError)
			}
			rightscripts[sequence] = append(rightscripts[sequence], *rs)
		}
	}

	return st, &rightscripts, errors
}

func ParseServerTemplate(ymlData io.Reader) (*ServerTemplate, error) {
	st := ServerTemplate{}
	bytes, err := ioutil.ReadAll(ymlData)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(bytes, &st)
	if err != nil {
		return nil, err
	}
	for sequence, _ := range st.RightScripts {
		if sequence != "Boot" && sequence != "Operational" && sequence != "Decommission" {
			typeError := fmt.Errorf("%s is not a valid sequence name for RightScripts.  Must be Boot, Operational, or Decommission:", sequence)
			return nil, typeError
		}
	}
	return &st, nil
}

func getServerTemplateByName(name string) (*cm15.ServerTemplate, error) {
	client := config.environment.Client15()

	stLocator := client.ServerTemplateLocator("/api/server_templates")
	apiParams := rsapi.APIParams{"filter": []string{"name==" + name}}
	fuzzySts, err := stLocator.Index(apiParams)
	if err != nil {
		return nil, err
	}
	var foundSt *cm15.ServerTemplate
	for _, st := range fuzzySts {
		if st.Name == name && st.Revision == 0 {
			if foundSt != nil {
				return nil, fmt.Errorf("Error, matched multiple ServerTemplates with the same name. Don't know which one to upload to. Please delete one of '%s'", name)
			} else {
				foundSt = st
			}
		}
	}
	return foundSt, nil
}
