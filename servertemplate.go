package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rightscale/rsc/cm15"
	"github.com/rightscale/rsc/rsapi"
	"gopkg.in/yaml.v2"
)

type ServerTemplate struct {
	href             string
	Name             string                    `yaml:"Name"`
	Description      string                    `yaml:"Description"`
	Inputs           map[string]*InputValue    `yaml:"Inputs"`
	RightScripts     map[string][]*RightScript `yaml:"RightScripts"`
	MultiCloudImages []*MultiCloudImage        `yaml:"MultiCloudImages"`
	Alerts           []*Alert                  `yaml:"Alerts"`
}

var (
	nullServerTemplate          = &cm15.ServerTemplate{}
	sequenceTypes      []string = []string{"Boot", "Operational", "Decommission"}
)

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
		stName := st.Name
		if prefix != "" {
			stName = fmt.Sprintf("%s_%s", prefix, stName)
		}
		fmt.Printf("Validation successful, uploading as '%s'\n", stName)

		if *debug {
			fmt.Printf("ST: %#v\n", *st)
		}
		err := doServerTemplateUpload(st, prefix)

		if err != nil {
			fatalError("Failed to upload ServerTemplate '%s': %s", file, err.Error())
		}
	}
}

func stDelete(files []string, prefix string) {
	client, _ := Config.Account.Client15()

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			fatalError("Cannot open file: %s", err.Error())
		}
		defer f.Close()

		st, err := ParseServerTemplate(f)
		if err != nil {
			fatalError("Cannot parse file: %s", err.Error())
		}

		// ServerTemplate first, then dependent parts
		stName := st.Name
		if prefix != "" {
			stName = fmt.Sprintf("%s_%s", prefix, st.Name)
		}
		hrefs, err := paramToHrefs("server_templates", stName, 0)
		if err != nil {
			fatalError("Could not query for ServerTemplates to delete: %s", err.Error())
		}
		if len(hrefs) == 0 {
			fmt.Printf("ServerTemplate '%s' does not exist, no need to delete\n", stName)
		}
		for _, href := range hrefs {
			loc := client.ServerTemplateLocator(href)
			fmt.Printf("Deleting ServerTemplate '%s' HREF %s\n", stName, href)
			err := loc.Destroy()
			if err != nil {
				fatalError("Failed to delete ServerTemplate %s: %s\n", stName, err.Error())
			}
		}

		// MultiCloudImages. Only delete ones managed by us and not simply ones we link to.
		for _, mciDef := range st.MultiCloudImages {
			if len(mciDef.Settings) > 0 {
				mciName := mciDef.Name
				if prefix != "" {
					mciName = fmt.Sprintf("%s_%s", prefix, mciDef.Name)
				}
				err := deleteMultiCloudImage(mciName)
				if err != nil {
					fmt.Printf("Failed to delete MultiCloudImage %s: %s\n", mciName, err.Error())
				}
			}
		}

		// RightScripts. Only delete ones managed by us and not simple ones we link to.
		seen := map[string]bool{}
		for _, scripts := range st.RightScripts {
			for _, rs := range scripts {
				if rs.Type == LocalRightScript {
					if !seen[rs.Path] {
						seen[rs.Path] = true
						err := deleteRightScript(filepath.Join(filepath.Dir(file), rs.Path), prefix)
						if err != nil {
							fmt.Printf("Failed to delete RightScript %s: %s\n", rs.Path, err.Error())
						}
					}
				}
			}
		}
	}

}

// Options:
//   -- commit
func doServerTemplateUpload(stDef *ServerTemplate, prefix string) error {
	client, _ := Config.Account.Client15()

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
	} else {
		if st.Description != stDef.Description {
			err := st.Locator(client).Update(&cm15.ServerTemplateParam{Description: stDef.Description})
			if err != nil {
				fatalError("Failed to update ServerTemplate '%s' description: %s", stName, err.Error())
			}
		}
	}
	stDef.href = getLink(st.Links, "self")
	fmt.Printf("%s ServerTemplate with HREF %s\n", stVerb, stDef.href)

	// -----------------
	// Synchronize MCIs
	// -----------------
	// Get a list of MCIs on the existing ST.
	fmt.Println("Updating MCIs:")
	if err := uploadMultiCloudImages(stDef, prefix); err != nil {
		fatalError("  Synchronize MultiCloudImages failed: %s", err.Error())
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
	// Remove RightScripts that don't belong from the sequence list. We must remove first else we might get an
	// error adding the same revision to a ST.
	for _, rb := range existingRbs {
		seenExistingRb := false
		for _, sequenceType := range sequenceTypes {
			for _, script := range stDef.RightScripts[sequenceType] {
				scriptHref := hrefByName[script.Metadata.Name]
				rbHref := getLink(rb.Links, "right_script")
				if rb.Sequence == strings.ToLower(sequenceType) && rbHref == scriptHref {
					seenExistingRb = true
				}
			}
		}
		if !seenExistingRb {
			fmt.Printf("  Removing %s from ServerTemplate %s bundle\n", getLink(rb.Links, "right_script"), rb.Sequence)
			err := rb.Locator(client).Destroy()
			if err != nil {
				fatalError("  Could not destroy RunnableBinding %s: %s", getLink(rb.Links, "right_script"), err.Error())
			}
		}
	}
	// Add RightScripts to the sequence list, if they're not there.
	for _, sequenceType := range sequenceTypes {
		for _, script := range stDef.RightScripts[sequenceType] {
			seenScript := false
			scriptHref := hrefByName[script.Metadata.Name]
			for _, rb := range existingRbs {
				rbHref := getLink(rb.Links, "right_script")
				if rb.Sequence == strings.ToLower(sequenceType) && rbHref == scriptHref {
					seenScript = true
				}
			}
			if !seenScript {
				params := cm15.RunnableBindingParam{
					RightScriptHref: scriptHref,
					Sequence:        strings.ToLower(sequenceType),
				}
				fmt.Printf("  Adding %s to %s bundle\n", scriptHref, strings.ToLower(sequenceType))
				_, err := rbLoc.Create(&params)
				if err != nil {
					fatalError("  Could not create %s RunnableBinding for HREF %s: %s", sequenceType, scriptHref, err.Error())
				}
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
	inputsLoc := client.InputLocator(stDef.href + "/inputs")
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
	fmt.Println("Synchronizing Alerts")
	if err := uploadAlerts(stDef); err != nil {
		fatalError("  Synchronize alerts failed: %s", err.Error())
	}

	fmt.Printf("Successfully uploaded ServerTemplate %s with HREF %s\n", st.Name, stDef.href)

	// If the user requested a commit on changes, commit the ST. This will commit all RightScripts as well.
	return nil
}

// TBD
//   Show uncommitted changes
//   Show a list of previous revisions?
//   If we're not head, show a link to the head revision/lineage?
func stShow(href string) {
	client, _ := Config.Account.Client15()

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

func stDownload(href, downloadTo string, usePublished, useReferences, downloadMciSettings bool, scriptPath string, output io.Writer) (string, []*cm15.MultiCloudImage, []*cm15.RightScript, error) {
	client, _ := Config.Account.Client15()

	stLocator := client.ServerTemplateLocator(href)
	st, err := stLocator.Show(rsapi.APIParams{"view": "inputs_2_0"})

	if err != nil {
		return "", nil, nil, fmt.Errorf("Could not find ServerTemplate with href %v: %v", href, err)
	}

	if downloadTo == "" {
		downloadTo = cleanFileName(st.Name) + ".yml"
	} else if isDirectory(downloadTo) {
		downloadTo = filepath.Join(downloadTo, cleanFileName(st.Name)+".yml")
	}
	fmt.Fprintf(output, "Downloading '%s' to '%s'\n", st.Name, downloadTo)

	//-------------------------------------
	// MultiCloudImages
	//-------------------------------------
	mcis, apiMcis, err := downloadMultiCloudImages(st, downloadMciSettings, output)
	if err != nil {
		return "", nil, nil, fmt.Errorf("Could not get MCIs from API: %v", err)
	}

	//-------------------------------------
	// RightScripts
	//-------------------------------------
	rbLocator := client.RunnableBindingLocator(getLink(st.Links, "runnable_bindings"))
	rbs, err := rbLocator.Index(rsapi.APIParams{})
	if err != nil {
		return "", nil, nil, fmt.Errorf("Could not find attached RightScripts with href %v: %v", rbLocator.Href, err)
	}
	// sort runnable bindings by position
	sort.SliceStable(rbs, func(i, j int) bool { return rbs[i].Position < rbs[j].Position })
	rightScripts := make(map[string][]*RightScript)
	countBySequence := make(map[string]int)
	positionBySequence := make(map[string]map[int]int)
	seenRightscript := make(map[string]*RightScript)
	for _, rb := range rbs {
		sequence := strings.Title(rb.Sequence)
		if _, ok := positionBySequence[sequence]; !ok {
			positionBySequence[sequence] = make(map[int]int)
		}
		// map the RightScript's position number to the index in the corresponding rightScripts sequence slice
		positionBySequence[sequence][rb.Position] = countBySequence[sequence]
		countBySequence[sequence]++
		seenRightscript[getLink(rb.Links, "right_script")] = nil
	}
	for sequenceType, count := range countBySequence {
		rightScripts[sequenceType] = make([]*RightScript, count)
	}
	apiRightScripts := make([]*cm15.RightScript, 0, len(seenRightscript))
	fmt.Fprintf(output, "Downloading %d attached RightScripts:\n", len(seenRightscript))
	for _, rb := range rbs {
		rsHref := getLink(rb.Links, "right_script")
		if rsHref == "" {
			return "", nil, nil, fmt.Errorf("Could not download ServerTemplate, it has attached cookbook recipes, which are not supported by this tool")
		}

		sequence := strings.Title(rb.Sequence)

		if scr, ok := seenRightscript[rsHref]; ok && scr != nil {
			rightScripts[sequence][positionBySequence[sequence][rb.Position]] = scr
			continue
		}

		newScript := RightScript{
			Type: LocalRightScript,
			Path: cleanFileName(rb.RightScript.Name),
		}
		var apiScript *cm15.RightScript
		if usePublished || useReferences {
			// We repull the rightscript here to get the description field, which we need to break ties between
			// similarly named publications!
			rsLoc := client.RightScriptLocator(rsHref)
			rs, err := rsLoc.Show(rsapi.APIParams{})
			if err != nil {
				return "", nil, nil, fmt.Errorf("Could not get RightScript %v: %v", rsHref, err)
			}
			pub, err := findPublication("RightScript", rs.Name, rs.Revision, map[string]string{`Description`: rs.Description})
			if err != nil {
				return "", nil, nil, fmt.Errorf("Error finding publication: %v", err)
			}
			// when using references but not published, do not include publication info for RightScripts in the same account
			if pub != nil && !usePublished {
				submatches := lineage.FindStringSubmatch(rs.Lineage)
				if submatches == nil {
					panic(fmt.Errorf("Unexpected RightScript lineage format: %s", rs.Lineage))
				}
				accountID, err := strconv.Atoi(submatches[1])
				if err != nil {
					panic(err)
				}
				if accountID == Config.Account.Id {
					pub = nil
				}
			}
			if pub != nil {
				fmt.Fprintf(output, "Not downloading '%v' to disk, using Revision %v, Publisher '%v' from the MultiCloud Marketplace\n",
					rs.Name, formatRev(rs.Revision), pub.Publisher)
				newScript = RightScript{
					Type:      PublishedRightScript,
					Name:      pub.Name,
					Revision:  pub.Revision,
					Publisher: pub.Publisher,
				}
			} else if useReferences {
				fmt.Fprintf(output, "Not downloading '%v' to disk, using Revision %v\n", rs.Name, formatRev(rs.Revision))
				newScript = RightScript{
					Type:     PublishedRightScript,
					Name:     rs.Name,
					Revision: rs.Revision,
				}
			}
			apiScript = rs
		}

		rightScripts[sequence][positionBySequence[sequence][rb.Position]] = &newScript

		if newScript.Type == LocalRightScript {
			if scriptPath == "" {
				downloadedTo, rs, _, err := rightScriptDownload(rsHref, filepath.Dir(downloadTo), true, output)
				if err != nil {
					return "", nil, nil, err
				}
				newScript.Path = strings.TrimPrefix(downloadedTo, filepath.Dir(downloadTo)+string(filepath.Separator))
				apiScript = rs
			} else {
				// Create scripts directory
				err := os.MkdirAll(filepath.Join(filepath.Dir(downloadTo), scriptPath), 0755)
				if err != nil {
					return "", nil, nil, fmt.Errorf("Error creating directory: %v", err)
				}
				downloadedTo, rs, _, err := rightScriptDownload(rsHref, filepath.Join(filepath.Dir(downloadTo), scriptPath), true, output)
				if err != nil {
					return "", nil, nil, err
				}
				newScript.Path = strings.TrimPrefix(downloadedTo, filepath.Dir(downloadTo)+string(filepath.Separator))
				apiScript = rs
			}
		}
		seenRightscript[rsHref] = &newScript
		apiRightScripts = append(apiRightScripts, apiScript)
	}

	//-------------------------------------
	// Alerts
	//-------------------------------------
	alerts, err := downloadAlerts(st)
	if err != nil {
		return "", nil, nil, fmt.Errorf("Could not get Alerts from API: %v", err)
	}

	//-------------------------------------
	// Inputs
	//-------------------------------------
	stInputs := make(map[string]*InputValue)
	for _, inputHash := range st.Inputs {
		iv, err := parseInputValue(inputHash["value"])
		if err != nil {
			return "", nil, nil, fmt.Errorf("Error parsing input value from API: %v", err)
		}
		// The API returns "inherit" values as "blank" values. Blank really means an
		// empty text string, which is usually not what was meant -- usually people
		// didn't mean to set the input at the ST level, in which case the
		// RightScript sets the value. Note that the user can still set the input to
		// blank at the RightScript level so it isn't much of a limitation this
		// occassionally doesn't always get set correctly on download
		// TBD: this can be improved slightly -- we can cross check the input with
		// the rightscript it came form. If its the same, we inherit. If we can
		// assume the ST overrides and use that value.
		if iv.Type != "blank" {
			stInputs[inputHash["name"]] = iv
		}
	}

	//-------------------------------------
	// ServerTemplate YAML itself finally
	//-------------------------------------
	stDef := ServerTemplate{
		Name:             st.Name,
		Description:      removeCarriageReturns(st.Description),
		Inputs:           stInputs,
		MultiCloudImages: mcis,
		RightScripts:     rightScripts,
		Alerts:           alerts,
	}
	bytes, err := yaml.Marshal(&stDef)
	if err != nil {
		return "", nil, nil, fmt.Errorf("Creating yaml failed: %v", err)
	}
	err = ioutil.WriteFile(downloadTo, bytes, 0644)
	if err != nil {
		return "", nil, nil, fmt.Errorf("Could not create file: %v", err)
	}
	fmt.Fprintf(output, "Finished downloading '%s' to '%s'\n", st.Name, downloadTo)

	return downloadTo, apiMcis, apiRightScripts, nil
}

func stValidate(files []string) {
	err_encountered := false
	for _, file := range files {
		_, errors := validateServerTemplate(file)
		if len(errors) != 0 {
			fmt.Println("Encountered the following errors with the ServerTemplate:")
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

func stDiff(href, revisionA, revisionB string, linkOnly bool, cache Cache) {
	stA, err := getServerTemplateRevision(href, revisionA)
	if err != nil {
		fatalError("Could not find revision-a: %v", err)
	}
	stB, err := getServerTemplateRevision(href, revisionB)
	if err != nil {
		fatalError("Could not find revision-b: %v", err)
	}

	if linkOnly {
		fmt.Println(getServerTemplateDiffLink(stA, stB))
	} else {
		differ, err := diffServerTemplate(os.Stdout, stA, stB, false, cache)
		if err != nil {
			fatalError("Error performing diff: %v", err)
		}
		if differ {
			os.Exit(1)
		}
	}
}

// diffServerTemplate returns whether two ServerTemplate revisions differ and writes the differences to w
func diffServerTemplate(w io.Writer, stA, stB *cm15.ServerTemplate, ignoreHeads bool, cache Cache) (bool, error) {
	yamlA, mcisA, scriptsA, dirA, err := getServerTemplateFiles(stA, cache)
	if err != nil {
		return false, err
	}
	defer yamlA.Close()
	if stA.Revision == 0 {
		defer os.RemoveAll(dirA)
	}
	yamlB, mcisB, scriptsB, dirB, err := getServerTemplateFiles(stB, cache)
	if err != nil {
		return false, err
	}
	defer yamlB.Close()
	if stB.Revision == 0 {
		defer os.RemoveAll(dirB)
	}

	if ignoreHeads {
		b, err := ioutil.ReadAll(yamlA)
		if err != nil {
			return false, err
		}
		yamlA.Close()

		st := &ServerTemplate{}
		if err := yaml.Unmarshal(b, st); err != nil {
			return false, err
		}

		for _, mci := range mcisB {
			if mci.Revision == 0 {
				for i, stMCI := range st.MultiCloudImages {
					if stMCI.Name == mci.Name {
						st.MultiCloudImages[i].Revision = 0
					}
				}
			}
		}

		for _, script := range scriptsB {
			if script.Revision == 0 {
				for sequence, stScripts := range st.RightScripts {
					for i, stScript := range stScripts {
						if stScript.Name == script.Name {
							st.RightScripts[sequence][i].Revision = 0
						}
					}
				}
			}
		}

		b, err = yaml.Marshal(st)
		if err != nil {
			return false, err
		}
		yamlA = ioutil.NopCloser(bytes.NewReader(b))
	}

	nameRevA := getServerTemplateNameRev(stA)
	nameRevB := getServerTemplateNameRev(stB)
	differ, output, err := Diff(nameRevA, nameRevB, time.Time{}, time.Time{}, yamlA, yamlB)
	if err != nil {
		return false, err
	}

	// line up the MCI lists by inserting null MCI entries for missing attachments
	for i := 0; i < len(mcisA) || i < len(mcisB); i++ {
		switch {
		case i >= len(mcisA):
			mcisA = append(mcisA, nil)
		case i >= len(mcisB):
			mcisB = append(mcisB, nil)
		case mcisA[i].Name < mcisB[i].Name:
			mcisB = append(mcisB[:i], append([]*cm15.MultiCloudImage{nullMCI}, mcisB[i:]...)...)
		case mcisA[i].Name > mcisB[i].Name:
			mcisA = append(mcisA[:i], append([]*cm15.MultiCloudImage{nullMCI}, mcisA[i:]...)...)
		}
	}

	// line up the RightScript lists by inserting null RightScript entries for missing attachments
	for i := 0; i < len(scriptsA) || i < len(scriptsB); i++ {
		switch {
		case i >= len(scriptsA):
			scriptsA = append(scriptsA, nullRightScript)
		case i >= len(scriptsB):
			scriptsB = append(scriptsB, nullRightScript)
		case scriptsA[i].Name < scriptsB[i].Name:
			scriptsB = append(scriptsB[:i], append([]*cm15.RightScript{nullRightScript}, scriptsB[i:]...)...)
		case scriptsA[i].Name > scriptsB[i].Name:
			scriptsA = append(scriptsA[:i], append([]*cm15.RightScript{nullRightScript}, scriptsA[i:]...)...)
		}
	}

	outputs := make([]string, 0, len(scriptsA)+len(mcisA))
	for i := 0; i < len(mcisA); i++ {
		// TODO diff MCIs
	}
	for i := 0; i < len(scriptsA); i++ {
		o := &strings.Builder{}
		d, err := diffRightScript(o, scriptsA[i], scriptsB[i], cache)
		if err != nil {
			return false, err
		}
		if d {
			outputs = append(outputs, o.String())
			if !differ {
				differ = true
			}
		}
	}

	if differ {
		fmt.Fprintf(w, "%v and %v differ\n\n%v\n\n", nameRevA, nameRevB, getServerTemplateDiffLink(stA, stB))
		fmt.Fprint(w, output)
		for _, o := range outputs {
			fmt.Fprint(w, o)
		}
	}

	return differ, nil
}

// getServerTemplateDiffLink returns the RightScale Dashboard URL for a diff between two ServerTemplate revisions
func getServerTemplateDiffLink(stA, stB *cm15.ServerTemplate) string {
	if getServerTemplateId(stA) == "" {
		return fmt.Sprintf("https://%v/acct/%v/server_templates/%v", Config.Account.Host, Config.Account.Id, getServerTemplateId(stB))
	}
	if getServerTemplateId(stB) == "" {
		return fmt.Sprintf("https://%v/acct/%v/server_templates/%v", Config.Account.Host, Config.Account.Id, getServerTemplateId(stA))
	}
	return fmt.Sprintf("https://%v/acct/%v/server_templates/diff?old_server_template_id=%v&new_server_template_id=%v", Config.Account.Host, Config.Account.Id, getServerTemplateId(stA), getServerTemplateId(stB))
}

// getServerTemplateFiles retreives the local path of a cached ServerTemplate and its directory
func getServerTemplateFiles(st *cm15.ServerTemplate, cache Cache) (reader io.ReadCloser, mcis []*cm15.MultiCloudImage, scripts []*cm15.RightScript, dir string, err error) {
	id := getServerTemplateId(st)
	if id == "" {
		reader = ioutil.NopCloser(&bytes.Reader{})
		return
	}
	var yaml string
	if st.Revision == 0 {
		dir, err = ioutil.TempDir("", "right_st.cache.")
		if err != nil {
			return
		}
		yaml = filepath.Join(dir, "server_template.yaml")
	} else {
		var cst *CachedServerTemplate
		cst, err = cache.GetServerTemplate(Config.Account.Id, id, st.Revision)
		if err != nil {
			return
		}
		if cst != nil {
			yaml = cst.File
			mcis, scripts = make([]*cm15.MultiCloudImage, len(cst.MultiCloudImages)), make([]*cm15.RightScript, len(cst.RightScripts))
			copy(mcis, cst.MultiCloudImages)
			copy(scripts, cst.RightScripts)
			goto finish
		}
		yaml, err = cache.GetServerTemplateFile(Config.Account.Id, id, st.Revision)
		if err != nil {
			return
		}
	}

	_, mcis, scripts, err = stDownload(getServerTemplateHREF(st), yaml, false, true, false, "", ioutil.Discard)
	if err != nil {
		return
	}

	if st.Revision != 0 {
		err = cache.PutServerTemplate(Config.Account.Id, id, st.Revision, st, mcis, scripts)
		if err != nil {
			return
		}
	}

finish:
	if dir == "" {
		dir = filepath.Dir(yaml)
	}
	sort.Slice(mcis, func(i, j int) bool {
		if mcis[i].Name == mcis[j].Name {
			return mcis[i].Revision < mcis[j].Revision
		}
		return mcis[i].Name < mcis[j].Name
	})
	sort.Slice(scripts, func(i, j int) bool {
		if scripts[i].Name == scripts[j].Name {
			return scripts[i].Revision < scripts[j].Revision
		}
		return scripts[i].Name < scripts[j].Name
	})
	reader, err = os.Open(yaml)
	return
}

// getServerTemplateRevision returns the ServerTemplate object for the given ServerTemplate HREF and revision;
// the revision may be "null", "head", "latest", or a number
func getServerTemplateRevision(href, revision string) (*cm15.ServerTemplate, error) {
	var (
		r   int
		err error
	)
	switch strings.ToLower(revision) {
	case "null":
		return nullServerTemplate, nil
	case "head":
		r = 0
	case "latest":
		r = -1
	default:
		r, err = strconv.Atoi(revision)
		if err != nil {
			return nil, err
		}
	}

	client, _ := Config.Account.Client15()
	locator := client.ServerTemplateLocator(href)

	// show the ServerTemplate to find its lineage
	st, err := locator.Show(rsapi.APIParams{})
	if err != nil {
		return nil, err
	}

	// get all of the ServerTemplates in the lineage
	stRevisions, err := locator.Index(rsapi.APIParams{"filter": []string{"lineage==" + st.Lineage}})
	if err != nil {
		return nil, err
	}

	// find the ServerTemplate in the lineage with the matching revision
	// or find the index of the ServerTemplate with the maximum revision if looking for "latest"
	maxI := -1
	for i, stRevision := range stRevisions {
		if r < 0 {
			if maxI < 0 || stRevision.Revision > stRevisions[maxI].Revision {
				maxI = i
			}
		} else if stRevision.Revision == r {
			return stRevision, nil
		}
	}

	// the maximum index will only be greater than -1 if looking for "latest"
	if maxI >= 0 {
		return stRevisions[maxI], nil
	}

	return nil, fmt.Errorf("no ServerTemplate found for %v with revision %v", href, revision)
}

// getServerTemplateId returns the ID of the ServerTemplate object
func getServerTemplateId(st *cm15.ServerTemplate) string {
	parts := strings.Split(getServerTemplateHREF(st), "/")
	return parts[len(parts)-1]
}

// getServerTemplateHREF returns the HREF of the RightScript object
func getServerTemplateHREF(st *cm15.ServerTemplate) string {
	client, _ := Config.Account.Client15()
	locator := st.Locator(client)
	if locator != nil {
		return string(locator.Href)
	}
	return ""
}

// getServerTemplateNameRev returns the name and revision of the ServerTemplate object
func getServerTemplateNameRev(st *cm15.ServerTemplate) string {
	if getServerTemplateId(st) == "" {
		return os.DevNull
	}
	if st.Revision == 0 {
		return st.Name + " [HEAD]"
	}
	return fmt.Sprintf("%v [rev %v]", st.Name, st.Revision)
}

// TBD
//   Handle Cookbooks in some way (error out)
func validateServerTemplate(file string) (*ServerTemplate, []error) {
	root := filepath.Dir(file)
	f, err := os.Open(file)
	if err != nil {
		return nil, []error{err}
	}
	defer f.Close()

	st, err := ParseServerTemplate(f)
	if err != nil {
		return nil, []error{err}
	}
	st.MultiCloudImages, err = ExpandMultiCloudImages(root, st.MultiCloudImages)
	if err != nil {
		return nil, []error{err}
	}
	st.Alerts, err = ExpandAlerts(root, st.Alerts)
	if err != nil {
		return nil, []error{err}
	}

	var errors []error

	//-------------------------------------
	// MultiCloudImages
	//-------------------------------------
	for _, mciDef := range st.MultiCloudImages {
		errors = append(errors, validateMultiCloudImage(mciDef)...)
	}

	//-------------------------------------
	// RightScripts
	//-------------------------------------
	for sequence, scripts := range st.RightScripts {
		for i, rs := range scripts {
			if rs.Type == PublishedRightScript {
				if rs.Publisher != "" {
					pub, err := findPublication("RightScript", rs.Name, rs.Revision, map[string]string{`Publisher`: rs.Publisher})

					if err != nil {
						errors = append(errors, fmt.Errorf("Error finding publication for RightScript: %s\n", err.Error()))
					}
					if pub == nil {
						errors = append(errors, fmt.Errorf("Could not find a publication in the MultiCloud Marketplace for RightScript '%s' Revision %s Publisher '%s'", rs.Name, formatRev(rs.Revision), rs.Publisher))
					} else {
						rs.Metadata.Description = pub.Description
					}
				} else {
					script, err := findRightScript(rs.Name, rs.Revision, map[string]string{})
					if err != nil {
						errors = append(errors, fmt.Errorf("Error finding RightScript: %s\n", err.Error()))
					}
					if script == nil {
						errors = append(errors, fmt.Errorf("Error finding RightScript '%s' Revision %s in account. Maybe add a Publisher?\n", rs.Name, formatRev(rs.Revision)))
					}
				}

				rs.Metadata.Name = rs.Name
			} else if rs.Type == LocalRightScript {
				rsNew, err := validateRightScript(filepath.Join(root, rs.Path), false)
				if err != nil {
					rsName := rs.Path
					if rsNew != nil {
						rsName = rsNew.Name
					}
					rsError := fmt.Errorf("RightScript error: %s - %s: %s", sequence, rsName, err.Error())
					errors = append(errors, rsError)
				}
				scripts[i] = rsNew
			}
		}
	}

	//-------------------------------------
	// Alerts
	//-------------------------------------
	for i, alert := range st.Alerts {
		err := validateAlert(alert)
		if err != nil {
			errors = append(errors, fmt.Errorf("Alert %d error: %s", i, err.Error()))
		}
	}

	return st, errors
}

func ParseServerTemplate(ymlData io.Reader) (*ServerTemplate, error) {
	st := &ServerTemplate{}
	bytes, err := ioutil.ReadAll(ymlData)
	if err != nil {
		return nil, err
	}
	err = yaml.UnmarshalStrict(bytes, st)
	if err != nil {
		return nil, err
	}
	for sequence, _ := range st.RightScripts {
		if sequence != "Boot" && sequence != "Operational" && sequence != "Decommission" {
			typeError := fmt.Errorf("%s is not a valid sequence name for RightScripts.  Must be Boot, Operational, or Decommission:", sequence)
			return nil, typeError
		}
	}
	return st, nil
}

func getServerTemplateByName(name string) (*cm15.ServerTemplate, error) {
	client, _ := Config.Account.Client15()

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

func stCommit(href, message string, force, commitHead, freezeRepos bool, cache Cache) bool {
	if !force {
		stLatest, err := getServerTemplateRevision(href, "latest")
		if err != nil {
			fatalError("Could not find latest: %v", err)
		}
		stHead, err := getServerTemplateRevision(href, "head")
		if err != nil {
			fatalError("Could not find head: %v", err)
		}

		if stLatest.Revision != 0 {
			differ, err := diffServerTemplate( /*ioutil.Discard*/ os.Stdout, stLatest, stHead, true, cache)
			if err != nil {
				fatalError("Error performing diff: %v", err)
			}
			if !differ {
				return false
			}
		}
	}

	client, _ := Config.Account.Client15()
	stLocator := client.ServerTemplateLocator(href)

	fmt.Printf("Committing Server Template %s\n", href)

	// https://reference.rightscale.com/api1.5/resources/ResourceServerTemplates.html#commit
	commitLocator, err := stLocator.Commit(strconv.FormatBool(commitHead), message, strconv.FormatBool(freezeRepos))
	if err != nil {
		fatalError("%v", err)
	}
	st, err := commitLocator.Show(rsapi.APIParams{})
	if err != nil {
		fatalError("%v", err)
	}
	fmt.Printf("Revision: %v\nHREF: %v\n", st.Revision, commitLocator.Href)

	return true
}
