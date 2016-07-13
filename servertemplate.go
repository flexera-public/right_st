package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-yaml/yaml"
	"github.com/rightscale/rsc/cm15"
	"github.com/rightscale/rsc/rsapi"
)

type MultiCloudImage struct {
	Href     string `yaml:"Href,omitempty"`
	Name     string `yaml:"Name,omitempty"`
	Revision int    `yaml:"Revision,omitempty"`
}

type ServerTemplate struct {
	Name             string                    `yaml:"Name"`
	Description      string                    `yaml:"Description"`
	Inputs           map[string]*InputValue    `yaml:"Inputs"`
	RightScripts     map[string][]*RightScript `yaml:"RightScripts"`
	MultiCloudImages []*MultiCloudImage        `yaml:"MultiCloudImages"`
	Alerts           []*Alert                  `yaml:"Alerts"`
}

type Alert struct {
	Name        string `yaml:"Name"`
	Description string `yaml:"Description,omitempty"`
	Clause      string `yaml:"Clause"`
}

var sequenceTypes []string = []string{"Boot", "Operational", "Decommission"}

func stUpload(files []string, prefix string) {

	for _, file := range files {
		fmt.Printf("Validating %s\n", file)
		st, errors := validateServerTemplate(file)
		if len(errors) != 0 {
			fmt.Println("Encountered the following errors with the ServerTemplate:")
			for _, err := range errors {
				fmt.Println(err)
			}
			os.Exit(1)
		}
		fmt.Printf("Validation successful, uploading as '%s'\n", st.Name)

		if *debug {
			fmt.Printf("ST: %#v\n", *st)
		}
		err := doServerTemplateUpload(*st, prefix)

		if err != nil {
			fatalError("Failed to upload ServerTemplate '%s': %s", file, err.Error())
		}
	}
}

// Options:
//   -- commit
func doServerTemplateUpload(stDef ServerTemplate, prefix string) error {
	client, err := Config.Account.Client15()
	if err != nil {
		return err
	}

	// Check if ST with same name (HEAD revisions only) exists. If it does, update the head
	stName := stDef.Name

	if prefix != "" {
		stName = fmt.Sprintf("%s_%s", prefix, stDef.Name)
	}
	st, err := getServerTemplateByName(stName)

	if err != nil {
		fatalError("Failed to query for ServerTemplate '%s': %s", stName, err.Error())
	}

	// -----------------
	// Synchronize ST
	// -----------------
	// st = ST cloud object. stDef = ST defined in YAML on disk
	stVerb := "Using"
	if st == nil {
		params := cm15.ServerTemplateParam{
			Description: stDef.Description,
			Name:        stName,
		}
		stLoc, err := client.ServerTemplateLocator("/api/server_templates").Create(&params)
		if err != nil {
			fatalError("Failed to create ServerTemplate '%s': %s", stName, err.Error())
		}
		st, err = stLoc.Show(rsapi.APIParams{})
		if err != nil {
			fatalError("Failed to refetch ServerTemplate '%s': %s", stLoc.Href, err.Error())
		}
		stVerb = "Creating"
	}
	stHref := getLink(st.Links, "self")
	fmt.Printf("%s ServerTemplate with HREF %s\n", stVerb, getLink(st.Links, "self"))

	// -----------------
	// Synchonize MCIs
	// -----------------
	// Get a list of MCIs on the existing ST.
	fmt.Println("Updating MCIs:")

	stMciLocator := client.ServerTemplateMultiCloudImageLocator("/api/server_template_multi_cloud_images")

	existingMcis, err := stMciLocator.Index(rsapi.APIParams{"filter": []string{"server_template_href==" + stHref}})
	if err != nil {
		fatalError("Could not find MCIs with href %s: %s", stMciLocator.Href, err.Error())
	}
	// Delete MCIs that exist on the existing ST but not in this definition. We perform all the deletions first so
	// we don't have to worry about readding the same MCI with a different revision and throwing an error later
	// firstValidMci is an MCI we know for sure we're not going to default. We can't delete an MCI marked default so
	// make the firstValidMci the default in that case then proceed with the delete.
	var firstValidMci *cm15.ServerTemplateMultiCloudImageLocator
	for _, mci := range existingMcis {
		mciHref := getLink(mci.Links, "multi_cloud_image")
		for _, mciDef := range stDef.MultiCloudImages {
			if mciDef.Href == mciHref {
				firstValidMci = mci.Locator(client)
				break
			}
		}
		if firstValidMci != nil {
			break
		}
	}
	// Dummy MCI. If we can't find ANY valid Mcis, that means we're going to delete all the existing attached MCIs
	// before adding any new ones. This presents a problem in that the API will return an error if you try and delete
	// the final MCI. We work around this by adding a dummy MCI we later delete
	if firstValidMci == nil {
		mciLocator := client.MultiCloudImageLocator("/api/multi_cloud_images")

		dummyMcis, err := mciLocator.Index(rsapi.APIParams{})
		if err != nil {
			fatalError("Failed to find dummy MCIs: %s", mciLocator.Href, err.Error())
		}
		params := cm15.ServerTemplateMultiCloudImageParam{
			MultiCloudImageHref: getLink(dummyMcis[0].Links, "self"),
			ServerTemplateHref:  stHref,
		}
		loc, err := stMciLocator.Create(&params)
		if err != nil {
			fatalError("  Failed to associate Dummy MCI '%s' with ServerTemplate '%s': %s", getLink(dummyMcis[0].Links, "self"), stHref, err.Error())
		}
		firstValidMci = loc
		defer loc.Destroy()
	}
	for _, mci := range existingMcis {
		mciHref := getLink(mci.Links, "multi_cloud_image")
		foundMci := false // found on ST definition
		for _, mciDef := range stDef.MultiCloudImages {
			if mciDef.Href == mciHref {
				foundMci = true
			}
		}
		if !foundMci {
			fmt.Printf("  Removing MCI %s\n", mciHref)
			if mci.IsDefault {
				firstValidMci.MakeDefault()
			}
			err := mci.Locator(client).Destroy()
			if err != nil {
				fatalError("  Could not Remove MCI %s", mciHref)
			}
		}
	}

	// Add all MCIs. If the MCI has not changed (HREF is the same, or combination of values is the same) don't update?
	for i, mciDef := range stDef.MultiCloudImages {
		foundMci := false // found on ST
		for _, mci := range existingMcis {
			mciHref := getLink(mci.Links, "multi_cloud_image")
			if mciDef.Href == mciHref {
				foundMci = true
			}
		}
		if !foundMci {
			params := cm15.ServerTemplateMultiCloudImageParam{
				MultiCloudImageHref: mciDef.Href,
				ServerTemplateHref:  stHref,
			}
			fmt.Printf("  Adding MCI '%s' revision '%d' (%s)\n", mciDef.Name, mciDef.Revision, mciDef.Href)
			loc, err := stMciLocator.Create(&params)
			if err != nil {
				fatalError("  Failed to associate MCI '%s' with ServerTemplate '%s': %s", mciDef.Href, stHref, err.Error())
			}
			if i == 0 {
				_ = loc.MakeDefault()
			}
		}
	}

	fmt.Println("  MCIs synced")

	// -----------------
	// Synchronize RightScripts
	// -----------------

	// By the time we get to here, we've done a good bit of error checking, so don't have to recheck much.
	// We know the files on disk are all openable, the RightScripts on disk have valid metadata.
	//   TBD: issue: we don't check for duplicate rightscript names till .Push(). move that check to validation section?
	// Get RightScript object in RightScale. RightScript.Push() handles both cases below:
	//		1. Doesn't exist: create
	//		2. Exists: Update contents
	fmt.Println("Updating or Creating RightScripts:")
	hrefByName := make(map[string]string)
	for _, sequenceType := range sequenceTypes {
		for _, script := range stDef.RightScripts[sequenceType] {
			if _, ok := hrefByName[script.Metadata.Name]; ok {
				continue
			}
			// Push() has the side effort of always populating script.Href which we use below -- probably
			// rework this to be a bit more upfront in the future.
			err := script.Push(prefix)
			hrefByName[script.Metadata.Name] = script.Href
			if err != nil {
				fatalError("  %s", err.Error())
			}
		}
	}
	fmt.Println("  RightScripts synced")

	// Add new RightScripts to the sequence list. Don't worry about order for now, that'll be fixed up below
	fmt.Println("Setting order of RightScripts:")
	rbLoc := client.RunnableBindingLocator(getLink(st.Links, "runnable_bindings"))
	existingRbs, _ := rbLoc.Index(rsapi.APIParams{})
	seenExistingRbs := make([]bool, len(existingRbs), len(existingRbs))
	for _, sequenceType := range sequenceTypes {
		for _, script := range stDef.RightScripts[sequenceType] {
			seenScript := false
			scriptHref := hrefByName[script.Metadata.Name]
			for i, rb := range existingRbs {
				rbHref := getLink(rb.Links, "right_script")
				if rb.Sequence == strings.ToLower(sequenceType) && rbHref == scriptHref {
					seenScript = true
					seenExistingRbs[i] = true
				}
			}
			if !seenScript {
				params := cm15.RunnableBindingParam{
					RightScriptHref: scriptHref,
					Sequence:        strings.ToLower(sequenceType),
				}
				fmt.Printf("  Adding %s to ServerTemplate %s bundle\n", scriptHref, sequenceType)
				_, err := rbLoc.Create(&params)
				if err != nil {
					fatalError("  Could not create %s RunnableBinding for HREF %s: %s", sequenceType, scriptHref, err.Error())
				}
			}
		}
	}
	// Remove RightScripts that don't belong from the sequence list
	for i, rb := range existingRbs {
		if !seenExistingRbs[i] {
			fmt.Printf("  Removing %s from ServerTemplate\n", getLink(rb.Links, "right_script"))
			err := rb.Locator(client).Destroy()
			if err != nil {
				fatalError("  Could not destroy RunnableBinding %s: %s", getLink(rb.Links, "right_script"), err.Error())
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

	bindings := []*cm15.RunnableBindings{}
	for _, sequenceType := range sequenceTypes {
		for i, script := range stDef.RightScripts[sequenceType] {
			key := strings.ToLower(sequenceType) + "_" + hrefByName[script.Metadata.Name]
			rb, ok := rbLookup[key]
			if !ok {
				fatalError("  Could not lookup RunnableBinding %s", key)
			}
			b := cm15.RunnableBindings{
				Id:       rb.Id,
				Position: fmt.Sprintf("%d", i+1),
				Sequence: rb.Sequence,
			}
			bindings = append(bindings, &b)
		}
	}
	if len(bindings) > 0 {
		err = rbLoc.MultiUpdate(bindings)
		if err != nil {
			fatalError("  MultiUpdate to set RunnableBinding order failed: %s", err.Error())
		}
		fmt.Println("  RightScript order set")
	} else {
		fmt.Println("  No RightScripts to order")
	}

	// -----------------
	// Set Inputs
	// -----------------
	fmt.Println("Setting Inputs")
	inputsLoc := client.InputLocator(stHref + "/inputs")
	oldInputs, err := inputsLoc.Index(rsapi.APIParams{"view": "inputs_2_0"})
	if err != nil {
		fatalError("  Failed to Index inputs: %s", err.Error())
	}
	inputParams := make(map[string]interface{})
	for _, input := range oldInputs {
		inputParams[input.Name] = "inherit"
	}
	for k, v := range stDef.Inputs {
		inputParams[k] = v.String()
	}
	if len(inputParams) > 0 {
		err = inputsLoc.MultiUpdate(inputParams)
		if err != nil {
			fatalError("  Failed to MultiUpdate inputs: %s", err.Error())
		}
		fmt.Println("  Inputs set")
	} else {
		fmt.Println("  No inputs to set")
	}

	// -----------------
	// Synchronize Alerts
	// -----------------
	fmt.Println("Synchonizing Alerts")
	alertsLocator := client.AlertSpecLocator(getLink(st.Links, "alert_specs"))
	existingAlerts, err := alertsLocator.Index(rsapi.APIParams{})
	if err != nil {
		fatalError("Could not find AlertSpecs with href %s: %s", alertsLocator.Href, err.Error())
	}
	seenAlert := make(map[string]bool)
	alertLookup := make(map[string]*cm15.AlertSpec)
	for _, alert := range existingAlerts {
		alertLookup[alert.Name] = alert
	}
	// Add/Update alerts
	for _, alert := range stDef.Alerts {
		parsedAlert, _ := parseAlertClause(alert.Clause)
		seenAlert[alert.Name] = true
		existingAlert, ok := alertLookup[alert.Name]
		if ok { // update
			if alert.Clause != printAlertClause(*existingAlert) || alert.Description != existingAlert.Description {
				alertsUpdateLocator := client.AlertSpecLocator(getLink(existingAlert.Links, "self"))

				fmt.Printf("  Updating Alert %s\n", alert.Name)
				params := cm15.AlertSpecParam2{
					Condition:      parsedAlert.Condition,
					Description:    alert.Description,
					Duration:       strconv.Itoa(parsedAlert.Duration),
					EscalationName: parsedAlert.EscalationName,
					File:           parsedAlert.File,
					Name:           alert.Name,
					Threshold:      parsedAlert.Threshold,
					Variable:       parsedAlert.Variable,
					VoteTag:        parsedAlert.VoteTag,
					VoteType:       parsedAlert.VoteType,
				}
				err := alertsUpdateLocator.Update(&params)
				if err != nil {
					fatalError("  Failed to update Alert %s: %s", alert.Name, err.Error())
				}
			}
		} else { // new alert
			fmt.Printf("  Adding Alert %s\n", alert.Name)
			params := cm15.AlertSpecParam{
				Condition:      parsedAlert.Condition,
				Description:    alert.Description,
				Duration:       strconv.Itoa(parsedAlert.Duration),
				EscalationName: parsedAlert.EscalationName,
				File:           parsedAlert.File,
				Name:           alert.Name,
				Threshold:      parsedAlert.Threshold,
				Variable:       parsedAlert.Variable,
				VoteTag:        parsedAlert.VoteTag,
				VoteType:       parsedAlert.VoteType,
			}
			_, err := alertsLocator.Create(&params)
			if err != nil {
				fatalError("  Failed to create Alert %s: %s", alert.Name, err.Error())
			}
		}
	}
	for _, alert := range existingAlerts {
		if !seenAlert[alert.Name] {
			fmt.Printf("  Removing alert %s\n", alert.Name)
			err := alert.Locator(client).Destroy()
			if err != nil {
				fatalError("  Could not destroy Alert %s: %s", alert.Name, err.Error())
			}
		}
	}

	fmt.Printf("Successfully uploaded ServerTemplate %s with HREF %s\n", st.Name, stHref)

	// If the user requested a commit on changes, commit the ST. This will commit all RightScripts as well.
	return nil
}

// TBD
//   Show uncommitted changes
//   Show a list of previous revisions?
//   If we're not head, show a link to the head revision/lineage?
func stShow(href string) {
	client, err := Config.Account.Client15()
	if err != nil {
		fatalError("Could not find ServerTemplate with href %s: %s", href, err.Error())
	}

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

	alertsLocator := client.AlertSpecLocator(getLink(st.Links, "alert_specs"))
	alerts, err := alertsLocator.Index(rsapi.APIParams{})
	if err != nil {
		fatalError("Could not find AlertSpecs with href %s: %s", alertsLocator.Href, err.Error())
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
	fmt.Printf("RightScripts:\n")
	seenSequence := make(map[string]bool)
	for _, sequenceType := range sequenceTypes {
		for _, item := range rbs {
			rsHref := getLink(item.Links, "right_script")
			//if item.RightScript != cm15.RightScript(nil) {
			rs := item.RightScript
			if item.Sequence != strings.ToLower(sequenceType) {
				continue
			}
			if !seenSequence[item.Sequence] {
				fmt.Printf("  %s: (href, rev, name)\n", sequenceType)
			}
			seenSequence[item.Sequence] = true
			rev := "HEAD"
			if rs.Revision != 0 {
				rev = fmt.Sprintf("%d", rs.Revision)
			}
			fmt.Printf("    %s %5s %s\n", rsHref, rev, rs.Name)
			// } else {
			//  fmt.Printf(" RECIPE - NOT HANDLED YET")
			// }
		}
	}
	fmt.Printf("Alerts:\n")
	for _, alert := range alerts {
		fmt.Printf("  - Name: %s\n", alert.Name)
		fmt.Printf("    Description: %s\n", alert.Description)
		fmt.Printf("    Value: %s\n", printAlertClause(*alert))

	}
}

func stDownload(href, downloadTo string, rsPath string, published bool) {
	client, err := Config.Account.Client15()
	if err != nil {
		fatalError("Could not find ServerTemplate with href %s: %s", href, err.Error())
	}

	stLocator := client.ServerTemplateLocator(href)
	st, err := stLocator.Show(rsapi.APIParams{"view": "inputs_2_0"})

	if err != nil {
		fatalError("Could not find ServerTemplate with href %s: %s", href, err.Error())
	}

	if downloadTo == "" {
		downloadTo = cleanFileName(st.Name) + ".yml"
	} else if isDirectory(downloadTo) {
		downloadTo = filepath.Join(downloadTo, cleanFileName(st.Name)+".yml")
	}
	fmt.Printf("Downloading '%s' to '%s'\n", st.Name, downloadTo)

	//-------------------------------------
	// MultiCloudImages
	//-------------------------------------
	mciLocator := client.MultiCloudImageLocator(getLink(st.Links, "multi_cloud_images"))
	mcis, err := mciLocator.Index(rsapi.APIParams{})
	if err != nil {
		fatalError("Could not find MCIs with href %s: %s", mciLocator.Href, err.Error())
	}
	mciImages := make([]*MultiCloudImage, len(mcis))
	for i, mci := range mcis {
		mciImages[i] = &MultiCloudImage{Name: mci.Name, Revision: mci.Revision}
	}

	//-------------------------------------
	// RightScripts
	//-------------------------------------
	rbLocator := client.RunnableBindingLocator(getLink(st.Links, "runnable_bindings"))
	rbs, err := rbLocator.Index(rsapi.APIParams{})
	if err != nil {
		fatalError("Could not find attached RightScripts with href %s: %s", rbLocator.Href, err.Error())
	}
	rightScripts := make(map[string][]*RightScript)
	countBySequence := make(map[string]int)
	for _, rb := range rbs {
		countBySequence[strings.Title(rb.Sequence)] += 1
	}
	for sequenceType, count := range countBySequence {
		rightScripts[sequenceType] = make([]*RightScript, count)
	}
	fmt.Printf("Downloading %d attached RightScripts:\n", len(rbs))
	for _, rb := range rbs {
		rsHref := getLink(rb.Links, "right_script")
		if rsHref == "" {
			fatalError("Could not download ServerTemplate, it has attached cookbook recipes, which are not supported by this tool.\n")
		}

		newScript := RightScript{
			Type: LocalRightScript,
			Path: cleanFileName(rb.RightScript.Name),
		}
		if published {
			// We repull the rightscript here to get the description field, which we need to break ties between
			// similarly named publications!
			rsLoc := client.RightScriptLocator(rsHref)
			rs, err := rsLoc.Show(rsapi.APIParams{})
			if err != nil {
				fatalError("Could not get RightScript %s: %s\n", rsHref, err.Error())
			}
			pub, err := findPublication("RightScript", rs.Name, rs.Revision, map[string]string{`Description`: rs.Description})
			if err != nil {
				fatalError("Error finding publication: %s\n", err.Error())
			}
			if pub != nil {
				fmt.Printf("Not downloading '%s' to disk, using Revision %d, Publisher '%s' from the marketplace\n",
					rs.Name, rs.Revision, pub.Publisher)
				newScript = RightScript{
					Type:      PublishedRightScript,
					Name:      pub.Name,
					Revision:  pub.Revision,
					Publisher: pub.Publisher,
				}
			}
		}

		rightScripts[strings.Title(rb.Sequence)][rb.Position-1] = &newScript

		if newScript.Type == LocalRightScript {
			if rsPath == "" {
				downloadedTo := rightScriptDownload(rsHref, filepath.Dir(downloadTo))
				newScript.Path = strings.TrimPrefix(downloadedTo, filepath.Dir(downloadTo)+string(filepath.Separator))
			} else {
                  		// Create parent directory
                  		err := os.MkdirAll(filepath.Join(filepath.Dir(downloadTo), rsPath), 0755)
                  		if err != nil {
                        		fatalError("Error creating directory: %s", err.Error())
                  		}
				//downloadedTo := rightScriptDownload(rsHref, filepath.Join(filepath.Dir(downloadTo),  rsPath))
				rightScriptDownload(rsHref, filepath.Join(filepath.Dir(downloadTo),  rsPath))
				// newScript.Path = strings.TrimPrefix(downloadedTo, filepath.Join(filepath.Dir(downloadTo), rsPath)+string(filepath.Separator))
				newScript.Path = rsPath + "/" + cleanFileName(rb.RightScript.Name)
			}
		}
	}

	//-------------------------------------
	// Alerts
	//-------------------------------------
	alertsLocator := client.AlertSpecLocator(getLink(st.Links, "alert_specs"))
	alertSpecs, err := alertsLocator.Index(rsapi.APIParams{})
	if err != nil {
		fatalError("Could not find Alerts with href %s: %s", alertsLocator.Href, err.Error())
	}
	alerts := make([]*Alert, len(alertSpecs))
	for i, alertSpec := range alertSpecs {
		alerts[i] = &Alert{
			Name:        alertSpec.Name,
			Description: alertSpec.Description,
			Clause:      printAlertClause(*alertSpec),
		}
	}

	//-------------------------------------
	// Inputs
	//-------------------------------------
	stInputs := make(map[string]*InputValue)
	for _, inputHash := range st.Inputs {
		iv, err := parseInputValue(inputHash["value"])
		if err != nil {
			fatalError("Error parsing input value from API:", err.Error())
		}
		stInputs[inputHash["name"]] = iv
	}

	//-------------------------------------
	// ServerTemplate YAML itself finally
	//-------------------------------------
	stDef := ServerTemplate{
		Name:             st.Name,
		Description:      st.Description,
		Inputs:           stInputs,
		MultiCloudImages: mciImages,
		RightScripts:     rightScripts,
		Alerts:           alerts,
	}
	bytes, err := yaml.Marshal(&stDef)
	if err != nil {
		fatalError("Creating yaml failed: %s", err.Error())
	}
	err = ioutil.WriteFile(downloadTo, bytes, 0644)
	if err != nil {
		fatalError("Could not create file: %s", err.Error())
	}
	fmt.Printf("Finished downloading '%s' to '%s'\n", st.Name, downloadTo)

}

func stValidate(files []string) {
	err_encountered := false
	for _, file := range files {
		_, errors := validateServerTemplate(file)
		if len(errors) != 0 {
			err_encountered = true
			for _, err := range errors {
				fmt.Fprintf(os.Stderr, "%s: %s\n", file, err.Error())
			}
		} else {
			fmt.Printf("%s: Valid ServerTemplate\n", file)
		}
	}
	if err_encountered {
		os.Exit(1)
	}
}

// TBD
//   Handle Cookbooks in some way (error out)
func validateServerTemplate(file string) (*ServerTemplate, []error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, []error{err}
	}
	defer f.Close()

	st, err := ParseServerTemplate(f)
	if err != nil {
		return nil, []error{err}
	}

	client, err := Config.Account.Client15()
	if err != nil {
		return nil, []error{err}
	}

	var errors []error

	//idMatch := regexp.MustCompile(`^\d+$`)
	// TBD: Let people specify MCIs multiple ways:
	//   1. Href
	//   2. Name/revision pair (preferred - at least somewhat portable)
	//   3. Set of Images + Tags to support autocreation/management of MCIs (TBD)
	for _, mciDef := range st.MultiCloudImages {
		if mciDef.Href != "" {
			loc := client.MultiCloudImageLocator(mciDef.Href)
			mci, err := loc.Show()
			if err != nil {
				errors = append(errors, fmt.Errorf("Could not find MCI HREF %s in account", mciDef.Href))
			}
			mciDef.Name = mci.Name
			mciDef.Revision = mci.Revision
		} else if mciDef.Name != "" {
			href, err := paramToHref("multi_cloud_images", mciDef.Name, mciDef.Revision)
			if err != nil {
				// TBD fallback: If revision != 0, look for this combo in publications and try that!
				errors = append(errors, fmt.Errorf("Could not find MCI named '%s' with revision %d in account", mciDef.Name, mciDef.Revision))
			}
			mciDef.Href = href
		} else {
			errors = append(errors, fmt.Errorf("MultiCloudImage item must be a hash with 'Name' and 'Revision' keys set to a valid values."))
			continue
		}
	}

	for sequence, scripts := range st.RightScripts {
		for i, rs := range scripts {
			if rs.Type == PublishedRightScript {
				matchers := map[string]string{}
				if rs.Publisher != "" {
					matchers[`Publisher`] = rs.Publisher
				}

				pub, err := findPublication("RightScript", rs.Name, rs.Revision, matchers)
				if err != nil {
					fatalError("Error finding publication: %s\n", err.Error())
				}
				if pub == nil {
					fatalError("Could not find a publication in library for RightScript '%s' Revision %d Publisher '%s'\n", rs.Name, rs.Revision, rs.Publisher)
				}
				rs.Metadata.Name = rs.Name
				rs.Metadata.Description = pub.Description
			} else if rs.Type == LocalRightScript {
				rsNew, err := validateRightScript(filepath.Join(filepath.Dir(file), rs.Path), false)
				if err != nil {
					rsError := fmt.Errorf("RightScript error: %s - %s: %s", sequence, rsNew.Name, err.Error())
					errors = append(errors, rsError)
				}
				scripts[i] = rsNew
			}
		}
	}

	for i, alert := range st.Alerts {
		if alert.Name == "" {
			alertError := fmt.Errorf("Alert %d error: Name field must be present", i)
			errors = append(errors, alertError)
		}
		_, err := parseAlertClause(alert.Clause)
		if err != nil {
			alertError := fmt.Errorf("Alert %d error: %s", i, err.Error())
			errors = append(errors, alertError)
		}
	}

	return st, errors
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
	client, err := Config.Account.Client15()
	if err != nil {
		return nil, err
	}

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

// Expected Format with array index offsets into tokens array below:
// If <Metric>.<ValueType> <ComparisonOperator> <Threshold> for <Duration> minutes Then <Escalate|Grow|Shrink> <ActionValue>
// 0  1                       2                    3           4   5          6       7    8											9
func parseAlertClause(alert string) (*cm15.AlertSpec, error) {
	alertSpec := new(cm15.AlertSpec)
	tokens := strings.SplitN(alert, " ", 10)
	alertFmt := `If <Metric>.<ValueType> <ComparisonOperator> <Threshold> for <Duration> minutes Then <Action> <ActionValue>`
	if len(tokens) != 10 {
		return nil, fmt.Errorf("Alert clause misformatted: not long enough. Must be of format: '%s'", alertFmt)
	}
	if strings.ToLower(tokens[0]) != "if" {
		return nil, fmt.Errorf("Alert clause misformatted: missing If. Must be of format: '%s'", alertFmt)
	}
	metricTokens := strings.Split(tokens[1], ".")
	if len(metricTokens) != 2 {
		return nil, fmt.Errorf("Alert <Metric>.<ValueType> misformatted, should be like 'cpu-0/cpu-idle.value'.")
	}
	// Check metricTokens[0] should contain a slash.
	// Check metricTokens[1] can be numerous types: count, cumulative_requests, current_session, free
	//   midterm, percent, processes, read, running, rx, tx, shortterm, state, status, threads,
	//   used, users, value, write
	alertSpec.File = metricTokens[0]
	alertSpec.Variable = metricTokens[1]
	comparisonValues := []string{">", ">=", "<", "<=", "==", "!="}
	foundValue := false
	for _, val := range comparisonValues {
		if tokens[2] == val {
			foundValue = true
		}
	}
	if !foundValue {
		return nil, fmt.Errorf("Alert <ComparisonOperator> must be one of the following comparison operators: %s", strings.Join(comparisonValues, ", "))
	}
	alertSpec.Condition = tokens[2]
	// Threshold must be one of NaN, numeric OR booting, decommission, operational, pending, stranded, terminated
	alertSpec.Threshold = tokens[3]
	if strings.ToLower(tokens[4]) != "for" {
		return nil, fmt.Errorf("Alert clause misformatted, missing 'for'. Must be of format: '%s'", alertFmt)
	}
	duration, err := strconv.Atoi(tokens[5])
	if err != nil || duration < 1 {
		return nil, fmt.Errorf("Alert <Duration> must be a positive integer > 0")
	}
	alertSpec.Duration = duration

	if strings.Trim(strings.ToLower(tokens[6]), ",") != "minutes" {
		return nil, fmt.Errorf("Alert clause misformatted: missing 'minutes'. Must be of format: '%s'", alertFmt)
	}
	if strings.ToLower(tokens[7]) != "then" {
		return nil, fmt.Errorf("Alert clause misformatted: missing 'Then'. Must be of format: '%s'", alertFmt)
	}
	token8 := strings.ToLower(tokens[8])
	if token8 != "escalate" && token8 != "grow" && token8 != "shrink" {
		return nil, fmt.Errorf("Alert <Action> must be escalate, grow, or shrink")
	}
	if token8 == "escalate" {
		alertSpec.EscalationName = tokens[9]
	} else {
		alertSpec.VoteType = token8
		alertSpec.VoteTag = tokens[9]
	}
	return alertSpec, nil
}

// Complement to parseAlertClause
// If <Metric>.<ValueType> <ComparisonOperator> <Threshold> for <Duration> minutes Then <Escalate|Grow|Shrink> <ActionValue>
// 0  1                       2                    3           4   5          6       7    8											9
func printAlertClause(as cm15.AlertSpec) string {
	var asAction, asActionValue string
	if as.EscalationName != "" {
		asAction = "escalate"
		asActionValue = as.EscalationName
	} else {
		asAction = as.VoteType
		asActionValue = as.VoteTag
	}
	alertStr := fmt.Sprintf("If %s.%s %s %s for %d minutes Then %s %s",
		as.File, as.Variable, as.Condition, as.Threshold, as.Duration, asAction, asActionValue)
	return alertStr
}
