package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
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

// RightScripts as saved in the YAML on disk come in two varieties:
// Type == local. This means that the source code of the RightScript is managed
//   locally on disk. Path will be populated with the location of the file
// Type == remote. This means that the source code lives in RightScale and is
//   merely linked here. A few different combinations are possible then:
//   (Name, Revision) or (Href)
const (
	LocalRightScript int = iota
	PublishedRightScript
)

type RightScript struct {
	Type      int // LocalRightScript or PublishedRightScript
	Href      string
	Path      string // Needed for local case
	Name      string // Needed for remote case
	Revision  int    // Needed for remote case
	Publisher string // Needed for remote case
	Metadata  RightScriptMetadata
}

func rightScriptShow(href string) {
	client, err := Config.Account.Client15()
	if err != nil {
		fatalError("Could not find rightscript with href %s: %s", href, err.Error())
	}

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
		fmt.Printf("  %s\n", i.Name)
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

// This can be improved to look for bash'isms for older style scripts, powershellisms, etc.
func guessExtension(source string) string {
	if matches := shebang.FindStringSubmatch(source); len(matches) > 0 {
		if strings.Contains(matches[0], "ruby") {
			return ".rb"
		} else if strings.Contains(matches[0], "perl") {
			return ".pl"
		} else if strings.Contains(matches[0], "sh") { // bash + sh
			return ".sh"
		}
	}

	return ""
}

func rightScriptDownload(href, downloadTo string) string {
	client, err := Config.Account.Client15()
	if err != nil {
		fatalError("Could not find RightScript with href %s: %s", href, err.Error())
	}

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
	sourceMetadata, err := ParseRightScriptMetadata(bytes.NewReader(source))
	if err != nil {
		fmt.Printf("WARNING: Metadata in %s is malformed: %s\n", rightscript.Name, err.Error())
	}

	attachments, err := attachmentsLocator.Index(rsapi.APIParams{})
	if err != nil {
		fatalError("Could get attachments for RightScript from href %s: %s", attachmentsHref, err.Error())
	}

	guessedExtension := guessExtension(string(source))

	if downloadTo == "" {
		downloadTo = cleanFileName(rightscript.Name) + guessedExtension
	} else if isDirectory(downloadTo) {
		downloadTo = filepath.Join(downloadTo, cleanFileName(rightscript.Name)+guessedExtension)
	}
	fmt.Printf("Downloading '%s' to '%s'\n", rightscript.Name, downloadTo)

	for i, attachment := range attachments {
		// API attachments are always just plain names without path information.
		// SourceMetadata attachment names may have path components describing where
		// to put the file on disk, thus are truthier, so we merge those in.
		if sourceMetadata != nil {
			for _, aSrc := range sourceMetadata.Attachments {
				if path.Base(attachment.Filename) == path.Base(aSrc) {
					attachments[i].Filename = aSrc
				}
			}
		}
	}
	downloadItems := []*downloadItem{}
	pathPrepend := filepath.Join(filepath.Dir(downloadTo), "attachments")
	for _, attachment := range attachments {
		var downloadLocations []string

		if filepath.IsAbs(attachment.Filename) {
			downloadLocations = []string{attachment.Filename}
		} else {
			// We have a primary filename and a backup filename to try in case there's
			// a conflict. Conflicts will arise when you have multiple RightScripts
			// attached to the same ServerTemplate with the same attachment name, i.e.
			// something generic like 'config.xml'
			downloadLocations = []string{
				filepath.Join(pathPrepend, attachment.Filename),
				filepath.Join(pathPrepend, cleanFileName(rightscript.Name), attachment.Filename),
			}
		}

		downloadUrl, err := url.Parse(attachment.DownloadUrl)
		if err != nil {
			fatalError("Could not parse URL of attachment: %s", err.Error())
		}
		downloadItem := downloadItem{
			url:       *downloadUrl,
			locations: downloadLocations,
			md5:       attachment.Digest,
		}
		downloadItems = append(downloadItems, &downloadItem)
	}
	if len(downloadItems) == 0 {
		fmt.Println("No attachments to download")
	} else {
		fmt.Printf("Download %d attachments:\n", len(downloadItems))
		err = downloadManager(downloadItems)
		if err != nil {
			fatalError("Failed to download all attachments: %s", err.Error())
		}
		for _, d := range downloadItems {
			for i, attachment := range attachments {
				if filepath.Base(attachment.Filename) == filepath.Base(d.downloadedTo) {
					attachments[i].Filename = strings.TrimLeft(d.downloadedTo, pathPrepend)
				}
			}
		}
	}

	inputs := InputMap{}
	for _, input := range rightscript.Inputs {
		inputs = append(inputs, jsonMapToInput(input))
	}
	attachmentNames := make([]string, len(attachments))
	for i, a := range attachments {
		attachmentNames[i] = a.Filename
	}
	apiMetadata := RightScriptMetadata{
		Name:        rightscript.Name,
		Description: rightscript.Description,
		Packages:    rightscript.Packages,
		Inputs:      inputs,
		Attachments: attachmentNames,
	}

	// Re-running it through scaffoldBuffer has the benefit of cleaning up any errors in how
	// the inputs are described. Also any attachments added or removed manually will be
	// handled in that the builtin metadata will reflect whats on disk
	scaffoldedSourceBytes, err := scaffoldBuffer(source, apiMetadata, "", false)
	if err == nil {
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

	return downloadTo
}

// Convert a JSON response to InputMetadata struct
func jsonMapToInput(input map[string]interface{}) InputMetadata {
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

	categoryName, _ := input["category_name"].(string) // May be null
	description, _ := input["description"].(string)    // May be null

	return InputMetadata{
		Name:           input["name"].(string),
		Category:       categoryName,
		Description:    description,
		InputType:      inputType,
		Required:       input["required"].(bool),
		Advanced:       input["advanced"].(bool),
		Default:        defaultValue,
		PossibleValues: possibleValues,
	}
}

func rightScriptScaffold(files []string, backup bool, force bool) {
	files, err := walkPaths(files)
	if err != nil {
		fatalError("%s\n", err.Error())
	}

	for _, file := range files {
		err = ScaffoldRightScript(file, backup, os.Stdout, force)
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
	client, err := Config.Account.Client15()
	if err != nil {
		return nil, err
	}

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
	client, err := Config.Account.Client15()
	if err != nil {
		return err
	}

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
	client, err := Config.Account.Client15()
	if err != nil {
		return "", err
	}

	createLocator := client.RightScriptLocator("/api/right_scripts")
	apiParams := rsapi.APIParams{"filter": []string{"name==" + name}}
	rightscripts, err := createLocator.Index(apiParams)
	if err != nil {
		return "", err
	}
	foundId := ""
	for _, rs := range rightscripts {
		// Recheck the name here, filter does a partial match and we need an exact one
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
	if r.Type == PublishedRightScript {
		return r.PushRemote()
	} else {
		return r.PushLocal(prefix)
	}
}

func (r *RightScript) PushRemote() error {
	client, err := Config.Account.Client15()
	if err != nil {
		return err
	}
	// Algorithm:
	//   1. Find the RightScript in publications first. Get the name/description/publisher
	//   2. If we don't find it, throw an error
	//   3. Get the imported RightScript. If it doesn't exist, import it then get the href
	//   4. Insert HREF into r struct for later use.
	matchers := map[string]string{}
	if r.Publisher != "" {
		matchers[`Publisher`] = r.Publisher
	}

	pub, err := findPublication("RightScript", r.Name, r.Revision, matchers)
	if err != nil {
		return err
	}
	if pub == nil {
		return fmt.Errorf("Could not find a publication in the MultiCloud Marketplace for RightScript '%s' Revision %d Publisher '%s'", r.Name, r.Revision, r.Publisher)
	}

	rsLocator := client.RightScriptLocator("/api/right_scripts")
	filters := []string{
		"name==" + r.Name,
	}

	rsUnfiltered, err := rsLocator.Index(rsapi.APIParams{"filter": filters})
	if err != nil {
		return err
	}
	for _, rs := range rsUnfiltered {
		// Recheck the name here, filter does a partial match and we need an exact one.
		// Matching the descriptions helps to disambiguate if we have multiple publications
		// with that same name/revision pair.
		if rs.Name == r.Name && rs.Revision == r.Revision && rs.Description == pub.Description {
			r.Href = getLink(rs.Links, "self")
		}
	}

	if r.Href == "" {
		loc := pub.Locator(client)

		err = loc.Import()

		if err != nil {
			return fmt.Errorf("Failed to import publication %s for RightScript '%s' Revision %d Publisher %s\n",
				getLink(pub.Links, "self"), r.Name, r.Revision, r.Publisher)
		}

		rsUnfiltered, err := rsLocator.Index(rsapi.APIParams{"filter": filters})
		if err != nil {
			return err
		}
		for _, rs := range rsUnfiltered {
			if rs.Name == r.Name && rs.Revision == r.Revision && rs.Description == pub.Description {
				r.Href = getLink(rs.Links, "self")
			}
		}
		if r.Href == "" {
			return fmt.Errorf("Could not refind RightScript '%s' Revision %d after import!", r.Name, r.Revision)
		}
	}

	return nil
}

func (r *RightScript) PushLocal(prefix string) error {
	client, err := Config.Account.Client15()
	if err != nil {
		return err
	}

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
			Packages:    r.Metadata.Packages,
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
			Packages:    r.Metadata.Packages,
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

	rightScript := RightScript{
		Type:     LocalRightScript,
		Href:     "",
		Path:     file,
		Name:     metadata.Name,
		Metadata: *metadata,
	}

	if *debug {
		pretty.Println(metadata)
	}

	if metadata.Inputs == nil {
		return &rightScript, fmt.Errorf("Inputs must be specified")
	}

	seenAttachments := make(map[string]bool)
	for _, attachment := range metadata.Attachments {
		if seenAttachments[path.Base(attachment)] {
			return nil, fmt.Errorf("Attachment name %s appears twice", attachment)
		}
		seenAttachments[path.Base(attachment)] = true
		// Support both relative and full paths
		fullPath := filepath.Join(filepath.Dir(file), "attachments", attachment)
		if filepath.IsAbs(attachment) {
			fullPath = attachment
		}

		file, err := os.Open(fullPath)
		if err != nil {
			return &rightScript, fmt.Errorf("Could not open attachment: %s. Make sure attachment is in \"attachments/\" subdirectory or an absolute path", err.Error())
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

func (rs RightScript) MarshalYAML() (interface{}, error) {
	if rs.Type == LocalRightScript {
		return rs.Path, nil
	} else {
		destMap := make(map[string]interface{})
		destMap["Name"] = rs.Name
		destMap["Revision"] = rs.Revision
		destMap["Publisher"] = rs.Publisher
		return destMap, nil
	}
}

func (rs *RightScript) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var pathType string
	var mapType map[string]string
	errorMsg := "Could not unmarshal RightScript. Must be either a path to file on disk or a hash with a Name/Revision keys"
	err := unmarshal(&pathType)
	if err == nil {
		rs.Type = LocalRightScript
		rs.Path = pathType
	} else {
		err = unmarshal(&mapType)
		if err != nil {
			return fmt.Errorf(errorMsg)
		}
		name, ok := mapType["Name"]
		if !ok {
			return fmt.Errorf(errorMsg)
		}
		rs.Name = name
		publisher, ok := mapType["Publisher"]
		if ok {
			rs.Publisher = publisher
		}
		revStr, ok := mapType["Revision"]
		if !ok {
			return fmt.Errorf(errorMsg)
		}
		rev, err := strconv.Atoi(revStr)
		if err != nil {
			return fmt.Errorf("Revision must be an integer")
		}
		rs.Type = PublishedRightScript
		rs.Revision = rev
	}

	return nil
}
