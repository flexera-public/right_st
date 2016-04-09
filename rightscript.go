package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rightscale/rsc/cm15"
	"github.com/rightscale/rsc/rsapi"
	"github.com/tonnerre/golang-pretty"
)

type Iterable struct {
	Links    []map[string]string `json:"links,omitempty"`
	Name     string              `json:"name,omitempty"`
	Revision int                 `json:"revision,omitempty"`
}

type RightScript struct {
	Href     string
	Path     string
	Metadata RightScriptMetadata
}

func rightScriptShow(href string) {
	client := Config.Account.Client15()

	attachmentsHref := fmt.Sprintf("%s/attachments", href)
	rightscriptLocator := client.RightScriptLocator(href)
	attachmentsLocator := client.RightScriptAttachmentLocator(attachmentsHref)

	rightscript, err := rightscriptLocator.Show(rsapi.APIParams{"view": "inputs_2_0"})
	if err != nil {
		fatalError("Could not find rightscript with href %s: %s", href, err.Error())
	}
	attachments, err := attachmentsLocator.Index(rsapi.APIParams{})
	if err != nil {
		fatalError("Could not find attachments with href %s: %s", attachmentsHref, err.Error())
	}
	source, err := getSource(rightscriptLocator)
	if err != nil {
		fatalError("Could get source for RightScript with href %s: %s", href, err.Error())
	}
	rev := "HEAD"
	if rightscript.Revision != 0 {
		rev = fmt.Sprintf("%d", rightscript.Revision)
	}
	fmt.Printf("Name: %s\n", rightscript.Name)
	fmt.Printf("HREF: /api/right_scripts/%s\n", rightscript.Id)
	fmt.Printf("Revision: %5s\n", rev)
	fmt.Printf("Inputs:\n")
	for _, input := range rightscript.Inputs {
		i := jsonMapToInput(input)
		fmt.Printf("  %s\n", input["name"].(string))
		fmt.Printf("    Category: %s\n", i.Category)
		fmt.Printf("    Description: %s\n", i.Description)
		fmt.Printf("    Input Type: %s\n", i.InputType.String())
		fmt.Printf("    Required: %t\n", i.Required)
		fmt.Printf("    Advanced: %t\n", i.Advanced)
		if i.Default != nil {
			fmt.Printf("    Default: %s\n", i.Default.String())
		}
		if len(i.PossibleValues) > 0 {
			vals := []string{}
			for _, pv := range i.PossibleValues {
				vals = append(vals, pv.String())
			}
			fmt.Printf("    Possible Values: %s\n", strings.Join(vals, ", "))
		}

	}
	fmt.Printf("Attachments (id, md5, name):\n")
	for _, a := range attachments {
		fmt.Printf("  %s %s %s\n", a.Id, a.Digest, a.Filename)
	}
	fmt.Println("Body:")
	fmt.Println(string(source))
}

func rightScriptUpload(files []string, force bool, prefix string) {
	// Pass 1, perform validations, gather up results
	scripts := []*RightScript{}
	files, err := walkPaths(files)
	if err != nil {
		fatalError("%s\n", err.Error())
		os.Exit(1)
	}

	for _, p := range files {
		fmt.Printf("Uploading %s\n", p)
		f, err := os.Open(p)
		if err != nil {
			fatalError("Cannot open %s", p)
		}
		defer f.Close()
		script, err := validateRightScript(p, force)
		if err != nil {
			fatalError("%s: %s\n", p, err.Error())
		}

		scripts = append(scripts, script)
	}

	// Pass 2, upload
	for _, script := range scripts {
		err = script.Push(prefix)
		if err != nil {
			fatalError("%s", err.Error())
		}
	}
}

func rightScriptDownload(href, downloadTo string) {
	client := Config.Account.Client15()

	attachmentsHref := fmt.Sprintf("%s/attachments", href)
	rightscriptLocator := client.RightScriptLocator(href)
	attachmentsLocator := client.RightScriptAttachmentLocator(attachmentsHref)

	rightscript, err := rightscriptLocator.Show(rsapi.APIParams{"view": "inputs_2_0"})
	if err != nil {
		fatalError("Could not find RightScript with href %s: %s", href, err.Error())
	}
	source, err := getSource(rightscriptLocator)
	if err != nil {
		fatalError("Could get source for RightScript with href %s: %s", href, err.Error())
	}

	attachments, err := attachmentsLocator.Index(rsapi.APIParams{})
	if err != nil {
		fatalError("Could get attachments for RightScript from href %s: %s", attachmentsHref, err.Error())
	}

	if downloadTo == "" {
		downloadTo = cleanFileName(rightscript.Name)
	} else if isDirectory(downloadTo) {
		downloadTo = filepath.Join(downloadTo, cleanFileName(rightscript.Name)+".yml")
	}
	fmt.Printf("Downloading '%s' to '%s'\n", rightscript.Name, downloadTo)

	attachmentNames := make([]string, len(attachments))
	for i, attachment := range attachments {
		attachmentNames[i] = attachment.Filename
	}

	inputs := make(InputMap)
	for _, input := range rightscript.Inputs {
		inputs[input["name"].(string)] = jsonMapToInput(input)
	}
	metadata := RightScriptMetadata{
		Name:        rightscript.Name,
		Description: rightscript.Description,
		Inputs:      &inputs,
		Attachments: attachmentNames,
	}

	scaffoldedSource, err := scaffoldBuffer(source, metadata, "")
	if err == nil {
		scaffoldedSourceBytes := scaffoldedSource.Bytes()
		if bytes.Compare(scaffoldedSourceBytes, source) != 0 {
			fmt.Println("Automatically inserted RightScript metadata.")
		}
		err = ioutil.WriteFile(downloadTo, scaffoldedSourceBytes, 0755)
	} else {
		fmt.Printf("Downloaded script as is. An error occurred generating metadata to insert into the RightScript: %s", err.Error())
		err = ioutil.WriteFile(downloadTo, source, 0755)
	}
	if err != nil {
		fatalError("Could not create file: %s", err.Error())
	}
	downloadItems := []*downloadItem{}
	for _, attachment := range attachments {
		attachmentPath := filepath.Join(filepath.Dir(downloadTo), "attachments", attachment.Filename)
		downloadUrl, err := url.Parse(attachment.DownloadUrl)
		if err != nil {
			fatalError("Could not parse URL of attachment: %s", err.Error())
		}
		downloadItems = append(downloadItems, &downloadItem{url: *downloadUrl, filename: attachmentPath, md5: attachment.Digest})
	}
	if len(downloadItems) == 0 {
		fmt.Println("No attachments to download")
	} else {
		fmt.Printf("Download %d attachments:\n", len(downloadItems))
		err = downloadManager(downloadItems)
		if err != nil {
			fatalError("Failed to download all attachments: %s", err.Error())
		}
	}

}

// Convert a JSON response to InputMetadata struct
func jsonMapToInput(input map[string]interface{}) *InputMetadata {
	var defaultValue *InputValue
	if rawValue, ok := input["default_value"].(string); ok {
		defaultValue, _ = parseInputValue(rawValue)
	}
	possibleValues := []*InputValue{}
	if rawPossibleValues, ok := input["possible_values"].([]interface{}); ok {
		for _, rawValue := range rawPossibleValues {
			possibleValue, _ := parseInputValue(rawValue.(string))
			possibleValues = append(possibleValues, possibleValue)
		}
	}

	inputType, _ := parseInputType(input["kind"].(string))

	return &InputMetadata{
		Category:       input["category_name"].(string),
		Description:    input["description"].(string),
		InputType:      inputType,
		Required:       input["required"].(bool),
		Advanced:       input["advanced"].(bool),
		Default:        defaultValue,
		PossibleValues: possibleValues,
	}
}

func rightScriptScaffold(files []string, backup bool) {
	files, err := walkPaths(files)
	if err != nil {
		fatalError("%s\n", err.Error())
	}

	for _, file := range files {
		err = ScaffoldRightScript(file, backup, os.Stdout)
		if err != nil {
			fatalError("%s\n", err.Error())
		}
	}
}

func rightScriptValidate(files []string) {

	err_encountered := false
	for _, file := range files {
		_, err := validateRightScript(file, true)
		if err != nil {
			err_encountered = true
			fmt.Fprintf(os.Stderr, "%s: %s\n", file, err.Error())
		} else {
			fmt.Printf("%s: Valid metadata\n", file)
		}
	}
	if err_encountered {
		os.Exit(1)
	}
}

// Crappy workaround. RSC doesn't return the body of the http request which contains
// the script source, so do the same lower level calls it does to get it.
func getSource(loc *cm15.RightScriptLocator) (respBody []byte, err error) {
	var params rsapi.APIParams
	var p rsapi.APIParams
	APIVersion := "1.5"
	client := Config.Account.Client15()

	uri, err := loc.ActionPath("RightScript", "show_source")
	if err != nil {
		return respBody, err
	}
	req, err := client.BuildHTTPRequest(uri.HTTPMethod, uri.Path, APIVersion, params, p)
	if err != nil {
		return respBody, err
	}
	resp, err := client.PerformRequest(req)
	if err != nil {
		return respBody, err
	}
	defer resp.Body.Close()
	respBody, _ = ioutil.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return respBody, fmt.Errorf("invalid response %s: %s", resp.Status, string(respBody))
	}
	return respBody, nil
}

// Crappy workaround 2. the RightScriptAttachmentLocator.Create call doesn't work
// because RSCs countless concrete types screw things up. The RSC create call calls BuildHttpRequest
// with the type passed in, which is serializes to JSON. Under different code paths
// (such as here or the command line) it passes in rsapi.APIParams instead of a fixed type of
// cm15.RightScriptAttachmentParams. BuildHTTPRequest has code to iterate over APIParams and
// turn it into a a multipart mime doc if it sees a FileUpload type. But it doesn't have
// code knowing about every concrete type to handle that.
func uploadAttachment(loc *cm15.RightScriptAttachmentLocator,
	file *rsapi.FileUpload, name string) error {
	var params rsapi.APIParams
	var p rsapi.APIParams
	APIVersion := "1.5"
	client := Config.Account.Client15()

	p_inner := rsapi.APIParams{
		"content":  file,
		"filename": name,
	}
	p = rsapi.APIParams{
		"right_script_attachment": p_inner,
	}
	uri, err := loc.ActionPath("RightScriptAttachment", "create")
	if err != nil {
		return err
	}
	req, err := client.BuildHTTPRequest(uri.HTTPMethod, uri.Path, APIVersion, params, p)
	if err != nil {
		return err
	}
	resp, err := client.PerformRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("invalid response %s: %s", resp.Status, string(respBody))
	}
	return nil
}

func rightScriptIdByName(name string) (string, error) {
	client := Config.Account.Client15()
	createLocator := client.RightScriptLocator("/api/right_scripts")
	apiParams := rsapi.APIParams{"filter": []string{"name==" + name}}
	rightscripts, err := createLocator.Index(apiParams)
	if err != nil {
		return "", err
	}
	foundId := ""
	for _, rs := range rightscripts {
		// Recheck the name here, filter does a impartial match and we need an exact one
		if rs.Name == name && rs.Revision == 0 {
			if foundId != "" {
				return "", fmt.Errorf("Error, matched multiple RightScripts with the same name, please delete one: %s %s", rs.Id, foundId)
			} else {
				foundId = rs.Id
			}
		}
	}
	return foundId, nil
}

func (r *RightScript) Push(prefix string) error {
	client := Config.Account.Client15()
	createLocator := client.RightScriptLocator("/api/right_scripts")
	scriptName := r.Metadata.Name
	if prefix != "" {
		scriptName = fmt.Sprintf("%s_%s", prefix, r.Metadata.Name)
	}
	foundId, err := rightScriptIdByName(scriptName)
	if err != nil {
		return err
	}

	fileSrc, err := ioutil.ReadFile(r.Path)
	if err != nil {
		return err
	}

	var rightscriptLocator *cm15.RightScriptLocator
	if foundId == "" {
		fmt.Printf("  Creating a new RightScript named '%s' from %s\n", scriptName, r.Path)
		// New one, perform create call
		params := cm15.RightScriptParam2{
			Name:        scriptName,
			Description: r.Metadata.Description,
			Source:      string(fileSrc),
		}
		rightscriptLocator, err = createLocator.Create(&params)
		if err != nil {
			return err
		}
		fmt.Printf("    RightScript created with HREF %s\n", rightscriptLocator.Href)
		r.Href = string(rightscriptLocator.Href)
	} else {
		// Found existing, do an update
		href := fmt.Sprintf("/api/right_scripts/%s", foundId)
		fmt.Printf("  Updating existing RightScript named '%s' with HREF %s from %s\n", scriptName, href, r.Path)

		params := cm15.RightScriptParam3{
			Name:        scriptName,
			Description: r.Metadata.Description,
			Source:      string(fileSrc),
		}
		rightscriptLocator = client.RightScriptLocator(href)
		err = rightscriptLocator.Update(&params)
		if err != nil {
			return err
		}
		r.Href = href
	}

	attachmentsHref := fmt.Sprintf("%s/attachments", rightscriptLocator.Href)
	attachmentsLocator := client.RightScriptAttachmentLocator(attachmentsHref)
	attachments, err := attachmentsLocator.Index(rsapi.APIParams{})
	if err != nil {
		return err
	}

	toUpload := make(map[string]string)                           // scripts we want to upload
	onRightscript := make(map[string]*cm15.RightScriptAttachment) // scripts attached to the rightsript
	for _, a := range r.Metadata.Attachments {
		fullPath := filepath.Join(filepath.Dir(r.Path), "attachments", a)
		md5, err := fmd5sum(fullPath)
		if err != nil {
			return err
		}
		// We use a compound key with the name+md5 here to work around a couple corner cases
		//   - if the file is renamed, it'll be deleted and reuploaded
		//   - if two files have the same md5 for whatever reason they won't clash
		toUpload[path.Base(a)+"_"+md5] = a
	}
	for _, a := range attachments {
		onRightscript[path.Base(a.Filename)+"_"+a.Digest] = a
	}

	// Two passes. First pass we delete RightScripts. This comes up when a file was
	// removed from the RightScript, or when the contents of a file on disk changed.
	// In the second case, the second pass will reupload the correct attachment.
	for digestKey, a := range onRightscript {
		if _, ok := toUpload[digestKey]; !ok {
			loc := a.Locator(client)

			fmt.Printf("  Deleting attachment '%s' with HREF '%s'\n", a.Filename, loc.Href)
			err := loc.Destroy()
			if err != nil {
				return err
			}
		}
	}

	// Second pass, now upload any missing attachment and any attachments that were
	// deleted because we changed file contents.
	for digestKey, name := range toUpload {
		digestKeyParts := strings.Split(digestKey, "_")
		md5 := digestKeyParts[len(digestKeyParts)-1]
		if _, ok := onRightscript[digestKey]; ok {
			fmt.Printf("  Attachment '%s' already uploaded with md5 %s\n", name, md5)
			// TBD -- update if a.Name != name?
		} else {
			fullPath := filepath.Join(filepath.Dir(r.Path), "attachments", name)
			fmt.Printf("  Uploading attachment '%s' with md5 %s\n", name, md5)
			f, err := os.Open(fullPath)
			if err != nil {
				return err
			}
			// FileUpload represents payload fields that correspond to multipart file uploads.
			file := rsapi.FileUpload{Name: "right_script_attachment[content]", Reader: f, Filename: name}
			//params := cm15.RightScriptAttachmentParam{Content: &file, Name: a}
			err = uploadAttachment(attachmentsLocator, &file, path.Base(name))
			if err != nil {
				return err
			}
		}
	}

	return err
}

// Validates that a file has valid metadata, including attachments.
// No metadata is considered valid, although the RightScriptMetadata returned will
// be intialized to default values. A RightScriptMetadata struct might still be
// returned if there are errors if the metadata was partially specified.
func validateRightScript(file string, ignoreMissingMetadata bool) (*RightScript, error) {
	script, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer script.Close()

	metadata, err := ParseRightScriptMetadata(script)
	if err != nil {
		return nil, err
	}

	if metadata == nil {
		if ignoreMissingMetadata {
			metadata = new(RightScriptMetadata)
			scriptname := path.Base(file)
			scriptext := path.Ext(scriptname)
			scriptname = strings.TrimRight(scriptname, scriptext)
			metadata.Name = scriptname
		} else {
			return nil, fmt.Errorf("No embedded metadata for %s. Use --force to upload anyways.", file)
		}
	}
	rightScript := RightScript{"", file, *metadata}

	if *debug {
		pretty.Println(metadata)
	}

	if metadata.Inputs == nil {
		return &rightScript, fmt.Errorf("Inputs must be specified")
	}

	for _, attachment := range metadata.Attachments {
		// To make this easier on ourselves and mirror how things are stored in RightScale, attachments can't have any path
		// elements in them. This assures all attachments will be placed in an "attachments" subdir by convention.
		if path.Base(attachment) != attachment {
			return nil, fmt.Errorf("Attachment name invalid: %s. Attachment names can't contain slashes.", attachment)
		}
		fullPath := filepath.Join(filepath.Dir(file), "attachments", attachment)

		file, err := os.Open(fullPath)
		if err != nil {
			return &rightScript, fmt.Errorf("Could not open attachment: %s. Make sure attachment is in \"attachments/\" subdirectory", err.Error())
		}
		_, err = md5sum(file)
		file.Close()
		if err != nil {
			return &rightScript, err
		}

	}

	if metadata.Name == "" {
		return &rightScript, fmt.Errorf("Name must be specified")
	}

	return &rightScript, nil
}
