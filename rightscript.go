package main

import (
	"fmt"
	"io/ioutil"
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

func rightScriptList(filter string) {
	client := config.environment.Client15()

	rightscriptLocator := client.RightScriptLocator("/api/right_scripts")
	var apiParams = rsapi.APIParams{"filter": []string{"name==" + filter}}
	fmt.Printf("Listing %s:\n", filter)
	//log15.Info("Listing", "RightScript", filter)
	rightscripts, err := rightscriptLocator.Index(apiParams)
	if err != nil {
		fatalError("%s", err.Error())
	}
	for _, rs := range rightscripts {
		rev := "HEAD"
		if rs.Revision != 0 {
			rev = fmt.Sprintf("%d", rs.Revision)
		}
		fmt.Printf("/api/right_scripts/%s %5s %s\n", rs.Id, rev, rs.Name)
	}
}

func rightScriptShow(href string) {
	client := config.environment.Client15()

	attachmentsHref := fmt.Sprintf("%s/attachments", href)
	rightscriptLocator := client.RightScriptLocator(href)
	attachmentsLocator := client.RightScriptAttachmentLocator(attachmentsHref)

	rightscript, err := rightscriptLocator.Show()
	if err != nil {
		fatalError("Could not find rightscript with href %s: %s", href, err.Error())
	}
	attachments, err := attachmentsLocator.Index(rsapi.APIParams{})
	if err != nil {
		fatalError("Could not find attachments with href %s: %s", attachmentsHref, err.Error())
	}
	rev := "HEAD"
	if rightscript.Revision != 0 {
		rev = fmt.Sprintf("%d", rightscript.Revision)
	}
	fmt.Printf("Name: %s\n", rightscript.Name)
	fmt.Printf("HREF: /api/right_scripts/%s\n", rightscript.Id)
	fmt.Printf("Revision: %5s\n", rev)
	fmt.Printf("Attachments (id, md5, name):\n")
	for _, a := range attachments {
		fmt.Printf("  %s %s %s\n", a.Id, a.Digest, a.Filename)
	}
}

func rightScriptUpload(files []string, force bool) {
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
		err = script.Push()
		if err != nil {
			fatalError("%s", err.Error())
		}
	}
}

func rightScriptDownload(href, downloadTo string) {
	client := config.environment.Client15()

	rightscriptLocator := client.RightScriptLocator(href)
	// attachmentsLocator := client.RightScriptLocator(fmt.Sprintf("%s/attachments", href))

	rightscript, err := rightscriptLocator.Show()
	if err != nil {
		fatalError("Could not find RightScript with href %s: %s", href, err.Error())
	}
	source, err := getSource(rightscriptLocator)
	if err != nil {
		fatalError("Could get source for RightScript with href %s: %s", href, err.Error())
	}

	// attachments, err2 := attachmentsLocator.Index(rsapi.APIParams{})
	if downloadTo == "" {
		downloadTo = rightscript.Name
	}
	fmt.Printf("Downloading '%s' to %s\n", rightscript.Name, downloadTo)
	err = ioutil.WriteFile(downloadTo, source, 0755)
	if err != nil {
		fatalError("Could not create file: %s", err.Error())
	}
}

func rightScriptScaffold(files []string, backup bool) {
	files, err := walkPaths(files)
	if err != nil {
		fatalError("%s\n", err.Error())
	}

	for _, file := range files {
		err = scaffoldRightScript(file, backup)
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
	client := config.environment.Client15()

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
	client := config.environment.Client15()

	p_inner := rsapi.APIParams{
		"content": file,
		"name":    name,
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
	//fmt.Printf("%#v", resp.Header)
	//location := resp.Header.Get("Location")
	// if len(location) == 0 {
	//  return "", fmt.Errorf("Missing location header in response")
	// } else {
	//  return location, nil
	// }
}

func rightScriptIdByName(name string) (string, error) {
	client := config.environment.Client15()
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

func (r *RightScript) Push() error {
	client := config.environment.Client15()
	createLocator := client.RightScriptLocator("/api/right_scripts")
	foundId, err := rightScriptIdByName(r.Metadata.Name)
	if err != nil {
		return err
	}

	fileSrc, err := ioutil.ReadFile(r.Path)
	if err != nil {
		return err
	}

	var rightscriptLocator *cm15.RightScriptLocator
	if foundId == "" {
		fmt.Printf("  Creating a new RightScript named '%s' from %s\n", r.Metadata.Name, r.Path)
		// New one, perform create call
		params := cm15.RightScriptParam2{
			Name:        r.Metadata.Name,
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
		fmt.Printf("  Updating existing RightScript named '%s' with HREF %s from %s\n", r.Metadata.Name, href, r.Path)

		params := cm15.RightScriptParam3{
			Name:        r.Metadata.Name,
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
		fullPath := filepath.Join(filepath.Dir(r.Path), a)
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
			// HACK: self href for attachment is wrong for now. We calculate our own
			// below. We ca back this code out when its fixed
			// scriptHref := getLink(a.Links, "right_script)")
			// href := fmt.Sprintf("%s/attachments/%s", scriptHref, a.Id)
			// loc := client.RightScriptAttachmentLocator(href)
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
			fullPath := filepath.Join(filepath.Dir(r.Path), name)
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
		fullPath := filepath.Join(filepath.Dir(file), attachment)

		_, err := fmd5sum(fullPath)
		if err != nil {
			return &rightScript, err
		}
	}

	if metadata.Name == "" {
		return &rightScript, fmt.Errorf("Name must be specified")
	}

	return &rightScript, nil
}
