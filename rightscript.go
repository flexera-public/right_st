package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kr/pretty"
	"github.com/rightscale/rsc/cm15"
	"github.com/rightscale/rsc/rsapi"
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

var (
	lineage                = regexp.MustCompile(`/api/acct/(\d+)/right_scripts/.+$`)
	powershellAssignment   = regexp.MustCompile(`(?im)^\s*\$[a-z0-9_:]+\s*=`)
	powershellWriteCmdlets = regexp.MustCompile(`(?im)^\s*Write-(?:Debug|Error|EventLog|Host|Information|Output|Progress|Verbose|Warning)`)
)

func rightScriptShow(href string) {
	client, _ := Config.Account.Client15()

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

	var err error
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
func GuessExtension(source string) string {
	if matches := shebang.FindStringSubmatch(source); len(matches) > 0 {
		match := strings.ToLower(matches[0])
		if strings.Contains(match, "ruby") {
			return ".rb"
		} else if strings.Contains(match, "perl") {
			return ".pl"
		} else if strings.Contains(match, "powershell") {
			return ".ps1"
		} else if strings.Contains(match, "sh") { // bash + sh
			return ".sh"
		}
	}

	if powershellAssignment.MatchString(source) || powershellWriteCmdlets.MatchString(source) {
		return ".ps1"
	}

	return ""
}

func rightScriptDownload(href, downloadTo string, scaffold bool, output io.Writer) (string, []string, error) {
	client, _ := Config.Account.Client15()

	attachmentsHref := fmt.Sprintf("%s/attachments", href)
	rightscriptLocator := client.RightScriptLocator(href)
	attachmentsLocator := client.RightScriptAttachmentLocator(attachmentsHref)

	rightscript, err := rightscriptLocator.Show(rsapi.APIParams{"view": "inputs_2_0"})
	if err != nil {
		return "", nil, fmt.Errorf("Could not find RightScript with href %v: %v", href, err)
	}
	source, err := getSource(rightscriptLocator)
	if err != nil {
		return "", nil, fmt.Errorf("Could not get source for RightScript with href %v: %v", href, err)
	}
	sourceMetadata, err := ParseRightScriptMetadata(bytes.NewReader(source))
	if err != nil {
		fmt.Fprintf(output, "WARNING: Metadata in %v is malformed: %v\n", rightscript.Name, err)
	}

	attachments, err := attachmentsLocator.Index(rsapi.APIParams{})
	if err != nil {
		return "", nil, fmt.Errorf("Could not get attachments for RightScript from href %v: %v", attachmentsHref, err)
	}

	guessedExtension := GuessExtension(string(source))

	if downloadTo == "" {
		downloadTo = cleanFileName(rightscript.Name) + guessedExtension
	} else if isDirectory(downloadTo) {
		downloadTo = filepath.Join(downloadTo, cleanFileName(rightscript.Name)+guessedExtension)
	}
	fmt.Fprintf(output, "Downloading '%v' to '%v'\n", rightscript.Name, downloadTo)

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
	pathPrepend := filepath.Join(filepath.Dir(downloadTo), "attachments") + string(os.PathSeparator)
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
			return "", nil, fmt.Errorf("Could not parse URL of attachment: %v", err)
		}
		downloadItem := downloadItem{
			url:       *downloadUrl,
			locations: downloadLocations,
			md5:       attachment.Digest,
		}
		downloadItems = append(downloadItems, &downloadItem)
	}
	if len(downloadItems) == 0 {
		fmt.Fprintln(output, "No attachments to download")
	} else {
		fmt.Fprintf(output, "Download %d attachments:\n", len(downloadItems))
		err = downloadManager(downloadItems, output)
		if err != nil {
			return "", nil, fmt.Errorf("Failed to download all attachments: %v", err)
		}
		for _, d := range downloadItems {
			for i, attachment := range attachments {
				if filepath.Base(attachment.Filename) == filepath.Base(d.downloadedTo) {
					attachments[i].Filename = strings.Replace(d.downloadedTo, pathPrepend, ``, 1)
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
		Description: removeCarriageReturns(rightscript.Description),
		Packages:    rightscript.Packages,
		Inputs:      inputs,
		Attachments: attachmentNames,
	}

	if scaffold {
		// Re-running it through scaffoldBuffer has the benefit of cleaning up any errors in how
		// the inputs are described. Also any attachments added or removed manually will be
		// handled in that the builtin metadata will reflect whats on disk
		scaffoldedSourceBytes, err := scaffoldBuffer(source, apiMetadata, "", false)
		if err == nil {
			if bytes.Compare(scaffoldedSourceBytes, source) != 0 {
				fmt.Fprintln(output, "Automatically inserted RightScript metadata.")
			}
			err = ioutil.WriteFile(downloadTo, scaffoldedSourceBytes, 0755)
		} else {
			fmt.Fprintf(output, "Downloaded script as is. An error occurred generating metadata to insert into the RightScript: %v", err)
			err = ioutil.WriteFile(downloadTo, source, 0755)
		}
	} else {
		err = ioutil.WriteFile(downloadTo, source, 0755)
	}
	if err != nil {
		return "", nil, fmt.Errorf("Could not create file: %v", err)
	}

	return downloadTo, attachmentNames, nil
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

func rightScriptDiff(href, revisionA, revisionB string, linkOnly bool, cache Cache) {
	rsA, err := getRightScriptRevision(href, revisionA)
	if err != nil {
		fatalError("Could not find revision-a: %v", err)
	}
	rsB, err := getRightScriptRevision(href, revisionB)
	if err != nil {
		fatalError("Could not find revision-b: %v", err)
	}

	if linkOnly {
		fmt.Println(getRightScriptDiffLink(rsA, rsB))
	} else {
		differ, err := diffRightScript(os.Stdout, rsA, rsB, cache)
		if err != nil {
			fatalError("Error performing diff: %v", err)
		}
		if differ {
			os.Exit(1)
		}
	}
}

// diffRightScript returns whether two ServerTemplate revisions differ and writes the differences to w
func diffRightScript(w io.Writer, rsA, rsB *cm15.RightScript, cache Cache) (bool, error) {
	scriptA, attachmentsA, dirA, err := getRightScriptFiles(rsA, cache)
	if err != nil {
		return false, err
	}
	defer scriptA.Close()
	if rsA.Revision == 0 {
		defer os.RemoveAll(dirA)
	}
	scriptB, attachmentsB, dirB, err := getRightScriptFiles(rsB, cache)
	if err != nil {
		return false, err
	}
	defer scriptB.Close()
	if rsB.Revision == 0 {
		defer os.RemoveAll(dirB)
	}

	nameRevA := getRightScriptNameRev(rsA)
	nameRevB := getRightScriptNameRev(rsB)
	differ, output, err := Diff(nameRevA, nameRevB, rsA.UpdatedAt.Time, rsB.UpdatedAt.Time, scriptA, scriptB)
	if err != nil {
		return false, err
	}

	// line up the attachments lists by inserting /dev/null entries for missing attachments
	for i := 0; i < len(attachmentsA) || i < len(attachmentsB); i++ {
		switch {
		case i >= len(attachmentsA):
			attachmentsA = append(attachmentsA, os.DevNull)
		case i >= len(attachmentsB):
			attachmentsB = append(attachmentsB, os.DevNull)
		case attachmentsA[i] < attachmentsB[i]:
			attachmentsB = append(attachmentsB[:i], append([]string{os.DevNull}, attachmentsB[i:]...)...)
		case attachmentsA[i] > attachmentsB[i]:
			attachmentsA = append(attachmentsA[:i], append([]string{os.DevNull}, attachmentsA[i:]...)...)
		}
	}

	outputs := make([]string, 0, len(attachmentsA))
	for i := 0; i < len(attachmentsA); i++ {
		attachmentA := attachmentsA[i]
		if attachmentA != os.DevNull {
			attachmentA = filepath.Join(dirA, "attachments", attachmentA)
		}
		readerA, err := os.Open(attachmentA)
		if err != nil {
			return false, err
		}
		defer readerA.Close()
		attachmentB := attachmentsB[i]
		if attachmentB != os.DevNull {
			attachmentB = filepath.Join(dirB, "attachments", attachmentB)
		}
		readerB, err := os.Open(attachmentB)
		if err != nil {
			return false, err
		}

		d, o, err := Diff(attachmentsA[i], attachmentsB[i], rsA.UpdatedAt.Time, rsB.UpdatedAt.Time, readerA, readerB)
		if err != nil {
			return false, err
		}
		if d {
			outputs = append(outputs, o)
			if !differ {
				differ = true
			}
		}
	}

	if differ {
		fmt.Fprintf(w, "%v and %v differ\n\n%v\n\n", nameRevA, nameRevB, getRightScriptDiffLink(rsA, rsB))
		fmt.Fprint(w, output)
		for _, o := range outputs {
			fmt.Fprint(w, o)
		}
	}

	return differ, nil
}

// getRightScriptDiffLink returns the RightScale Dashboard URL for a diff between two RightScript revisions
func getRightScriptDiffLink(rsA, rsB *cm15.RightScript) string {
	return fmt.Sprintf("https://%v/acct/%v/right_scripts/diff?old_script_id=%v&new_script_id=%v", Config.Account.Host, Config.Account.Id, rsA.Id, rsB.Id)
}

// getRightScriptFiles retreives the local paths of a cached RightScript, its attachments, and the directory
func getRightScriptFiles(rs *cm15.RightScript, cache Cache) (reader io.ReadCloser, attachments []string, dir string, err error) {
	var script string
	if rs.Revision == 0 {
		dir, err = ioutil.TempDir("", "right_st.cache.")
		if err != nil {
			return
		}
		script = filepath.Join(dir, "right_script")
	} else {
		var crs *CachedRightScript
		crs, err = cache.GetRightScript(Config.Account.Id, rs.Id, rs.Revision)
		if err != nil {
			return
		}
		if crs != nil {
			script = crs.File
			attachments = make([]string, 0, len(crs.Attachments))
			for attachment := range crs.Attachments {
				attachments = append(attachments, attachment)
			}
			goto finish
		}
		script, err = cache.GetRightScriptFile(Config.Account.Id, rs.Id, rs.Revision)
		if err != nil {
			return
		}
	}

	_, attachments, err = rightScriptDownload(getRightScriptHREF(rs), script, false, ioutil.Discard)
	if err != nil {
		return
	}

	if rs.Revision != 0 {
		err = cache.PutRightScript(Config.Account.Id, rs.Id, rs.Revision, rs, attachments)
		if err != nil {
			return
		}
	}

finish:
	if dir == "" {
		dir = filepath.Dir(script)
	}
	sort.Strings(attachments)
	reader, err = os.Open(script)
	return
}

// getRightScriptRevision returns the RightScript object for the given RightScript HREF and revision;
// the revision may be "head", "latest", or a number
func getRightScriptRevision(href, revision string) (*cm15.RightScript, error) {
	var (
		r   int
		err error
	)
	switch strings.ToLower(revision) {
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
	locator := client.RightScriptLocator(href)

	// show the RightScript to find its lineage
	rs, err := locator.Show(rsapi.APIParams{})
	if err != nil {
		return nil, err
	}

	params := rsapi.APIParams{"filter": []string{"lineage==" + rs.Lineage}}
	if r < 0 {
		params["latest_only"] = "true"
	}

	// get all of the RightScripts in the lineage or just the latest if looking for "latest"
	rsRevisions, err := locator.Index(params)
	if err != nil {
		return nil, err
	}

	// find the RightScript in the lineage with the matching revision
	// or the only RightScript if looking for "latest"
	for _, rsRevision := range rsRevisions {
		if r < 0 || rsRevision.Revision == r {
			return rsRevision, nil
		}
	}

	return nil, fmt.Errorf("no RightScript found for %v with revision %v", href, revision)
}

// getRightScriptHREF returns the HREF of the RightScript object
func getRightScriptHREF(rs *cm15.RightScript) string {
	client, _ := Config.Account.Client15()
	return string(rs.Locator(client).Href)
}

// getRightScriptNameRev returns the name and revison of the RightScript object
func getRightScriptNameRev(rs *cm15.RightScript) string {
	if rs.Revision == 0 {
		return rs.Name + " [HEAD]"
	}
	return fmt.Sprintf("%v [rev %v]", rs.Name, rs.Revision)
}

func rightScriptDelete(files []string, prefix string) {
	for _, file := range files {
		err := deleteRightScript(file, prefix)
		if err != nil {
			fatalError("Could not delete RightScript %s: %s", err.Error())
		}
	}
}

func deleteRightScript(file string, prefix string) error {
	client, _ := Config.Account.Client15()

	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("Cannot open file: %s", err.Error())
	}
	defer f.Close()

	metadata, err := ParseRightScriptMetadata(f)
	if err != nil {
		return fmt.Errorf("Cannot parse RightScript metadata: %s", err.Error())
	}

	var scriptName string
	if metadata == nil {
		scriptName := path.Base(file)
		scriptExt := path.Ext(scriptName)
		scriptName = strings.TrimRight(scriptName, scriptExt)
	} else {
		scriptName = metadata.Name
	}
	if prefix != "" {
		scriptName = fmt.Sprintf("%s_%s", prefix, scriptName)
	}
	hrefs, err := paramToHrefs("right_scripts", scriptName, 0)
	if err != nil {
		return err
	}
	if len(hrefs) == 0 {
		fmt.Printf("RightScript '%s' does not exist, no need to delete\n", scriptName)
	}
	for _, href := range hrefs {
		loc := client.RightScriptLocator(href)
		fmt.Printf("Deleting RightScript '%s' HREF %s\n", scriptName, href)
		err := loc.Destroy()
		if err != nil {
			return err
		}
	}
	return nil
}

// Crappy workaround. RSC doesn't return the body of the http request which contains
// the script source, so do the same lower level calls it does to get it.
func getSource(loc *cm15.RightScriptLocator) (respBody []byte, err error) {
	var params rsapi.APIParams
	var p rsapi.APIParams
	APIVersion := "1.5"
	client, _ := Config.Account.Client15()

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
	client, _ := Config.Account.Client15()

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
	client, _ := Config.Account.Client15()

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
			// Only consider our own RightScripts when uploading
			submatches := lineage.FindStringSubmatch(rs.Lineage)
			if submatches == nil {
				panic(fmt.Errorf("Unexpected RightScript lineage format: %s", rs.Lineage))
			}
			accountId, err := strconv.Atoi(submatches[1])
			if err != nil {
				panic(err)
			}
			if accountId != Config.Account.Id {
				continue
			}
			if foundId != "" {
				return "", fmt.Errorf("Error, matched multiple RightScripts with the same name, please delete one: %s %s", rs.Id, foundId)
			} else {
				foundId = rs.Id
			}
		}
	}
	return foundId, nil
}

// Finds a RightScript in the local account
// Params:
//   name: name of RightScript to search for
//   revision: revision of the RightScript to search for. -1 means "latest"
//   matchers: Hash of string (field name) -> string (match value). additional matching criteria in case there are
//     multiple RightScripts with the same name and revision. Usually `Description` is used as a tie breaker.
// Returns:
//   RightScript if found. nil if not found. errors fatally if multiple RightScripts are found.
func findRightScript(name string, revision int, matchers map[string]string) (*cm15.RightScript, error) {
	client, _ := Config.Account.Client15()

	rsLocator := client.RightScriptLocator("/api/right_scripts")
	unfiltered, err := rsLocator.Index(rsapi.APIParams{"filter": []string{"name==" + name}})
	if err != nil {
		return nil, err
	}
	// Publisher handled a bit specially. Unfortunately, there is no Publisher field for a RightScript. We have a lineage
	// field has account ids in it. We have limited ability to match account ids to lineages unfortunately :(
	publisher, _ := matchers[`Publisher`]
	publishers := map[string]string{
		"RightScale":      "/2901/",
		"RightLink Agent": "/67972/",
	}
	publisher, _ = publishers[publisher]

	maxRevision := -1
	var scripts []*cm15.RightScript
	for _, rs := range unfiltered {
		// Recheck the name here, filter does a partial match and we need an exact one.
		// Matching the descriptions helps to disambiguate if we have multiple publications
		// with that same name/revision pair.
		if rs.Name == name {
			matched_all_matchers := true
			for fieldName, value := range matchers {
				if fieldName == `Publisher` {
					continue
				}
				v := reflect.Indirect(reflect.ValueOf(rs)).FieldByName(fieldName)
				if v.IsValid() {
					if v.String() != value {
						matched_all_matchers = false
					}
				}
			}

			if publisher != "" {
				if !strings.Contains(rs.Lineage, publisher) {
					matched_all_matchers = false
				}
			}
			if matched_all_matchers {
				// revision -1 means latest revision. We replace our list of found pubs with only the highest rev.
				if revision == -1 {
					if rs.Revision > maxRevision {
						maxRevision = rs.Revision
						scripts = []*cm15.RightScript{rs}
					}
				} else if rs.Revision == revision {
					scripts = append(scripts, rs)
				}
			}
		}
	}
	if len(scripts) == 0 {
		return nil, nil
	} else if len(scripts) == 2 {
		errMsg := fmt.Sprintf("Too many RightScripts matching %s with revision %s", name, formatRev(revision))
		fmt.Println(errMsg)
		for _, script := range scripts {
			href := getLink(script.Links, "self")
			fmt.Printf("  Script Href: %s\n", href)
		}
		return nil, fmt.Errorf("%s", errMsg)
	} else {
		return scripts[0], nil
	}
}

func (r *RightScript) Push(prefix string) error {
	if r.Type == PublishedRightScript {
		return r.PushRemote()
	} else {
		return r.PushLocal(prefix)
	}
}

func (r *RightScript) PushRemote() error {
	client, _ := Config.Account.Client15()

	// Algorithm:
	//   1. If a publisher is specified, find the RightScript in publications. Error if not found
	//     a. Get the imported RightScript. If it doesn't exist, import it then get the href
	//   2. If a publisher is not specified, check Local account. Error if not found
	//   3. Insert HREF into r struct for later use.
	// If this first part is changed, copy it to servertemplate.go validation section as well.
	var pub *cm15.Publication
	var err error
	pubMatcher := map[string]string{}
	rev := r.Revision
	if r.Publisher != "" {
		pub, err = findPublication("RightScript", r.Name, r.Revision, map[string]string{`Publisher`: r.Publisher})
		if err != nil {
			return err
		}
		if pub == nil {
			return fmt.Errorf("Could not find a publication in the MultiCloud Marketplace for RightScript '%s' Revision %s Publisher '%s'", r.Name, formatRev(r.Revision), r.Publisher)
		}
		pubMatcher[`Description`] = pub.Description
		pubMatcher[`Publisher`] = r.Publisher
		rev = pub.Revision
	}
	script, err := findRightScript(r.Name, rev, pubMatcher)
	if script == nil {
		if pub == nil {
			return fmt.Errorf("Could not find RightScript '%s' Revision %s in local account. Add a 'Publisher' to also search the MultiCloud Marketplace", r.Name, formatRev(r.Revision))
		} else {
			loc := pub.Locator(client)
			impLoc, err := loc.Import()
			if err != nil {
				return fmt.Errorf("Failed to import publication %s for RightScript '%s' Revision %s Publisher %s\n",
					getLink(pub.Links, "self"), r.Name, formatRev(rev), r.Publisher)
			}
			scriptLocator := client.RightScriptLocator(string(impLoc.Href))
			script, err = scriptLocator.Show(rsapi.APIParams{})
			if err != nil {
				return fmt.Errorf("Error looking up RightScript: %v", err)
			}
			r.Href = getLink(script.Links, "self")
		}
	} else {
		r.Href = getLink(script.Links, "self")
	}

	return nil
}

func (r *RightScript) PushLocal(prefix string) error {
	client, _ := Config.Account.Client15()

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
			scriptName := path.Base(file)
			scriptExt := path.Ext(scriptName)
			scriptName = strings.TrimRight(scriptName, scriptExt)
			metadata.Name = scriptName
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

func rightScriptCommit(href, message string) {
	client, _ := Config.Account.Client15()
	scriptLocator := client.RightScriptLocator(href)

	fmt.Printf("Committing RightScript %s\n", href)

	commitLocator, err := scriptLocator.Commit(&cm15.RightScriptParam{CommitMessage: message})
	if err != nil {
		fatalError("%v", err)
	}
	script, err := commitLocator.Show(rsapi.APIParams{})
	if err != nil {
		fatalError("%v", err)
	}
	fmt.Printf("Revision: %v\nHREF: %v\n", script.Revision, commitLocator.Href)
}

func (rs RightScript) MarshalYAML() (interface{}, error) {
	if rs.Type == LocalRightScript {
		return rs.Path, nil
	} else {
		destMap := make(map[string]interface{})
		destMap["Name"] = rs.Name
		if rs.Revision == -1 {
			destMap["Revision"] = "latest"
		} else {
			destMap["Revision"] = rs.Revision
		}
		destMap["Publisher"] = rs.Publisher
		return destMap, nil
	}
}

func (rs *RightScript) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var pathType string
	var mapType map[string]string
	errorMsg := "Could not unmarshal RightScript. Must be either a path to file on disk or a hash with Publisher/Name/Revision or Name/Revision keys"
	err := unmarshal(&pathType)
	if err == nil {
		rs.Type = LocalRightScript
		rs.Path = pathType
	} else {
		rs.Type = PublishedRightScript
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
			revStr = "0"
		}
		if revStr == "latest" {
			rs.Revision = -1
		} else if revStr == "head" {
			rs.Revision = 0
		} else {
			rev, err := strconv.Atoi(revStr)
			if err != nil {
				return fmt.Errorf("Revision must be an integer")
			}
			rs.Revision = rev
		}
	}

	return nil
}
