package main

//	"github.com/tonnerre/golang-pretty"

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/alecthomas/kingpin"
	"github.com/inconshreveable/log15"
	"github.com/mattn/go-colorable"
	"github.com/rightscale/rsc/cm15"
	"github.com/rightscale/rsc/httpclient"
	"github.com/rightscale/rsc/log"
	"github.com/rightscale/rsc/rsapi"
)

var (
	app        = kingpin.New("right_st", "A command-line application for managing RightScripts")
	debug      = app.Flag("debug", "Debug mode").Short('d').Bool()
	configFile = app.Flag("config", "Set the config file path.").Short('c').Default(DefaultConfigFile()).String()
	account    = app.Flag("account", "RightScale account name to use").Short('a').String()

	// ----- ServerTemplates -----
	stCmd = app.Command("st", "ServerTemplate")

	stShowCmd        = stCmd.Command("show", "Show a single ServerTemplate")
	stShowNameOrHref = stShowCmd.Arg("name|href|id", "ServerTemplate Name or HREF or Id").Required().String()

	stUploadCmd    = stCmd.Command("upload", "Upload a ServerTemplate specified by a YAML document")
	stUploadPaths  = stUploadCmd.Arg("path", "File or directory containing script files to upload").Required().ExistingFilesOrDirs()
	stUploadPrefix = stUploadCmd.Flag("prefix", "Create dev/test version by adding prefix to name of all ServerTemplate and RightScripts uploaded").Short('x').String()

	stDeleteCmd    = stCmd.Command("delete", "Delete dev/test ServerTemplates and RightScripts with a prefix")
	stDeletePaths  = stDeleteCmd.Arg("path", "File or directory containing script files").Required().ExistingFilesOrDirs()
	stDeletePrefix = stDeleteCmd.Flag("prefix", "Prefix to delete").Short('x').String()

	stDownloadCmd         = stCmd.Command("download", "Download a ServerTemplate and all associated RightScripts/Attachments to disk")
	stDownloadNameOrHref  = stDownloadCmd.Arg("name|href|id", "Script Name or HREF or Id").Required().String()
	stDownloadTo          = stDownloadCmd.Arg("path", "Download location").String()
	stDownloadPublished   = stDownloadCmd.Flag("published", "Insert links to published RightScripts instead of downloading to disk.").Short('p').Bool()
	stDownloadMciSettings = stDownloadCmd.Flag("mci-settings", "Download MCI settings data to recreate/manage an MCI.").Short('m').Bool()
	stDownloadScriptPath  = stDownloadCmd.Flag("script-path", "Download RightScripts and their attachments to a subdirectory relative to the download location.").Short('s').String()

	stValidateCmd   = stCmd.Command("validate", "Validate a ServerTemplate YAML document")
	stValidatePaths = stValidateCmd.Arg("path", "Path to script file(s)").Required().ExistingFiles()

	stCommitCmd              = stCmd.Command("commit", "Commit ServerTemplate")
	stCommitNameOrHrefOrPath = stCommitCmd.Arg("name|href|id|path", "ServerTemplate name, HREF, ID or file path").Required().Strings()
	stCommitMessage          = stCommitCmd.Flag("message", "ServerTemplate commit message").Short('m').Required().String()

	// ----- RightScripts -----
	rightScriptCmd = app.Command("rightscript", "RightScript")

	rightScriptShowCmd        = rightScriptCmd.Command("show", "Show a single RightScript and its attachments")
	rightScriptShowNameOrHref = rightScriptShowCmd.Arg("name|href|id", "Script Name or HREF or Id").Required().String()

	rightScriptUploadCmd    = rightScriptCmd.Command("upload", "Upload a RightScript")
	rightScriptUploadPaths  = rightScriptUploadCmd.Arg("path", "File or directory containing script files to upload").Required().ExistingFilesOrDirs()
	rightScriptUploadPrefix = rightScriptUploadCmd.Flag("prefix", "Create dev/test version by adding prefix to name of all RightScripts uploaded").Short('x').String()
	rightScriptUploadForce  = rightScriptUploadCmd.Flag("force", "Force upload of file if metadata is not present").Short('f').Bool()

	rightScriptDeleteCmd    = rightScriptCmd.Command("delete", "Delete dev/test RightScripts with a prefix.")
	rightScriptDeletePaths  = rightScriptDeleteCmd.Arg("path", "File or directory containing script files").Required().ExistingFilesOrDirs()
	rightScriptDeletePrefix = rightScriptDeleteCmd.Flag("prefix", "Prefix to delete").Short('x').String()

	rightScriptDownloadCmd        = rightScriptCmd.Command("download", "Download a RightScript to a file or files")
	rightScriptDownloadNameOrHref = rightScriptDownloadCmd.Arg("name|href|id", "Script Name or HREF or Id").Required().String()
	rightScriptDownloadTo         = rightScriptDownloadCmd.Arg("path", "Download location").String()

	rightScriptScaffoldCmd      = rightScriptCmd.Command("scaffold", "Add RightScript YAML metadata comments to a file or files")
	rightScriptScaffoldPaths    = rightScriptScaffoldCmd.Arg("path", "File or directory to set metadata for").Required().ExistingFilesOrDirs()
	rightScriptScaffoldNoBackup = rightScriptScaffoldCmd.Flag("no-backup", "Do not create backup files before scaffolding").Short('n').Bool()
	rightScriptScaffoldForce    = rightScriptScaffoldCmd.Flag("force", "Force re-scaffolding").Short('f').Bool()

	rightScriptValidateCmd   = rightScriptCmd.Command("validate", "Validate RightScript YAML metadata comments in a file or files")
	rightScriptValidatePaths = rightScriptValidateCmd.Arg("path", "Path to script file or directory containing script files").Required().ExistingFilesOrDirs()

	rightScriptCommitCmd              = rightScriptCmd.Command("commit", "Commit RightScript")
	rightScriptCommitNameOrHrefOrPath = rightScriptCommitCmd.Arg("name|href|id|path", "RightScript name, HREF, ID or file path").Required().Strings()
	rightScriptCommitMessage          = rightScriptCommitCmd.Flag("message", "RightScript commit message").Short('m').Required().String()

	// ----- Configuration -----
	configCmd = app.Command("config", "Manage Configuration")

	configAccountCmd     = configCmd.Command("account", "Add or edit configuration for a RightScale API account")
	configAccountName    = configAccountCmd.Arg("name", "Name of RightScale API Account to add or edit").Required().String()
	configAccountDefault = configAccountCmd.Flag("default", "Set the named RightScale API Account as the default").Short('D').Bool()

	configShowCmd = configCmd.Command("show", "Show configuration")

	// ----- Update right_st -----
	updateCmd = app.Command("update", "Update "+app.Name+" executable")

	updateListCmd = updateCmd.Command("list", "List any available updates for the "+app.Name+" executable")

	updateApplyCmd          = updateCmd.Command("apply", "Apply the latest update for the current major version or a specified major version")
	updateApplyMajorVersion = updateApplyCmd.Flag("major-version", "Major version to update to").Short('m').Int()
)

func main() {
	app.Writer(os.Stdout)
	app.Version(VV)
	app.HelpFlag.Short('h')
	app.VersionFlag.Short('v')
	command := kingpin.MustParse(app.Parse(os.Args[1:]))

	err := ReadConfig(*configFile, *account)
	if !strings.HasPrefix(command, "config") && !strings.HasPrefix(command, "update") {
		// Makes sure the config file structure is valid
		if err != nil {
			fatalError("%s: Error reading config file: %s\n", filepath.Base(os.Args[0]), err.Error())
		}

		// Make sure the config file auth token is valid. Check now so we don't have to
		// keep rechecking in code.
		_, err := Config.Account.Client15()
		if err != nil {
			fatalError("Authentication error: %s", err.Error())
		}
	}

	// Handle logging
	logLevel := log15.LvlInfo

	if *debug {
		log.Logger.SetHandler(
			log15.LvlFilterHandler(
				log15.LvlDebug,
				log15.StderrHandler))
		httpclient.DumpFormat = httpclient.Debug
		logLevel = log15.LvlDebug
	}
	handler := log15.LvlFilterHandler(logLevel, log15.StreamHandler(colorable.NewColorableStdout(), log15.TerminalFormat()))
	log15.Root().SetHandler(handler)

	if Config.GetBool("update.check") && !strings.HasPrefix(command, "update") {
		defer UpdateCheck(VV, os.Stderr)
	}

	switch command {
	case stShowCmd.FullCommand():
		href, err := paramToHref("server_templates", *stShowNameOrHref, 0, true)
		if err != nil {
			fatalError("%s", err.Error())
		}
		stShow(href)
	case stUploadCmd.FullCommand():
		files, err := walkPaths(*stUploadPaths)
		if err != nil {
			fatalError("%s\n", err.Error())
		}
		stUpload(files, *stUploadPrefix)
	case stDeleteCmd.FullCommand():
		files, err := walkPaths(*stDeletePaths)
		if err != nil {
			fatalError("%s\n", err.Error())
		}
		stDelete(files, *stDeletePrefix)
	case stDownloadCmd.FullCommand():
		href, err := paramToHref("server_templates", *stDownloadNameOrHref, 0, false)
		if err != nil {
			fatalError("%s", err.Error())
		}
		stDownload(href, *stDownloadTo, *stDownloadPublished, *stDownloadMciSettings, *stDownloadScriptPath)
	case stValidateCmd.FullCommand():
		files, err := walkPaths(*stValidatePaths)
		if err != nil {
			fatalError("%s\n", err.Error())
		}
		stValidate(files)
	case stCommitCmd.FullCommand():
		for _, input := range *stCommitNameOrHrefOrPath {
			href, err := paramToHref("server_templates", input, 0, true)
			if err != nil {
				fatalError("%s", err.Error())
			}
			stCommit(href, *stCommitMessage)
		}
	case rightScriptShowCmd.FullCommand():
		href, err := paramToHref("right_scripts", *rightScriptShowNameOrHref, 0, true)
		if err != nil {
			fatalError("%s", err.Error())
		}
		rightScriptShow(href)
	case rightScriptUploadCmd.FullCommand():
		files, err := walkPaths(*rightScriptUploadPaths)
		if err != nil {
			fatalError("%s\n", err.Error())
		}
		rightScriptUpload(files, *rightScriptUploadForce, *rightScriptUploadPrefix)
	case rightScriptDeleteCmd.FullCommand():
		files, err := walkPaths(*rightScriptDeletePaths)
		if err != nil {
			fatalError("%s\n", err.Error())
		}
		rightScriptDelete(files, *rightScriptDeletePrefix)
	case rightScriptDownloadCmd.FullCommand():
		href, err := paramToHref("right_scripts", *rightScriptDownloadNameOrHref, 0, false)
		if err != nil {
			fatalError("%s", err.Error())
		}
		rightScriptDownload(href, *rightScriptDownloadTo)
	case rightScriptScaffoldCmd.FullCommand():
		files, err := walkPaths(*rightScriptScaffoldPaths)
		if err != nil {
			fatalError("%s\n", err.Error())
		}
		rightScriptScaffold(files, !*rightScriptScaffoldNoBackup, *rightScriptScaffoldForce)
	case rightScriptValidateCmd.FullCommand():
		files, err := walkPaths(*rightScriptValidatePaths)
		if err != nil {
			fatalError("%s\n", err.Error())
		}
		rightScriptValidate(files)
	case rightScriptCommitCmd.FullCommand():
		for _, input := range *rightScriptCommitNameOrHrefOrPath {
			href, err := paramToHref("right_scripts", input, 0, true)
			if err != nil {
				fatalError("%s", err.Error())
			}
			rightScriptCommit(href, *rightScriptCommitMessage)
		}
	case configAccountCmd.FullCommand():
		err := Config.SetAccount(*configAccountName, *configAccountDefault, os.Stdin, os.Stdout)
		if err != nil {
			fatalError("%s\n", err.Error())
		}
	case configShowCmd.FullCommand():
		err := Config.ShowConfiguration(os.Stdout)
		if err != nil {
			fatalError("%s\n", err.Error())
		}
	case updateListCmd.FullCommand():
		err := UpdateList(VV, os.Stdout)
		if err != nil {
			fatalError("%s\n", err.Error())
		}
	case updateApplyCmd.FullCommand():
		err := UpdateApply(VV, os.Stdout, *updateApplyMajorVersion, "")
		if err != nil {
			fatalError("%s\n", err.Error())
		}
	}
}

// Distill a passed in user parameter (id or href or name) to hrefs. A name
// can correspond to multiple hrefs so an array of all matches is returned in
// that case.
func paramToHrefs(resourceType, param string, revision int) ([]string, error) {
	client, _ := Config.Account.Client15()

	idMatch := regexp.MustCompile(`^\d+$`)
	hrefMatch := regexp.MustCompile(fmt.Sprintf("^/api/%s/\\d+$", resourceType))

	var hrefs []string
	if idMatch.Match([]byte(param)) {
		hrefs = append(hrefs, fmt.Sprintf("/api/%s/%s", resourceType, param))
	} else if hrefMatch.Match([]byte(param)) {
		hrefs = append(hrefs, param)
	} else {
		payload := rsapi.APIParams{}
		params := rsapi.APIParams{"filter[]": []string{"name==" + param}}
		uriPath := fmt.Sprintf("/api/%s", resourceType)

		req, err := client.BuildHTTPRequest("GET", uriPath, "1.5", params, payload)
		if err != nil {
			return hrefs, err
		}
		resp, err := client.PerformRequest(req)
		if err != nil {
			return hrefs, err
		}
		defer resp.Body.Close()
		respBody, _ := ioutil.ReadAll(resp.Body)
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return hrefs, fmt.Errorf("invalid response %s: %s", resp.Status, string(respBody))
		}
		items := []Iterable{}
		err = json.Unmarshal(respBody, &items)
		if err != nil {
			return hrefs, err
		}
		for _, item := range items {
			if item.Name == param && item.Revision == revision {
				hrefs = append(hrefs, getLink(item.Links, "self"))
			}
		}
	}
	return hrefs, nil
}

// Distill a passed in user parameter (id or href or name) to a single href or
// else return an error.
func paramToHref(resourceType, param string, revision int, filePathInInput bool) (string, error) {
	var (
		hrefs []string
		err   error
	)

	// if filePathInInput is true, we have to go through an extra computation
	// to retrieve the script name from the metadata of the file
	if filePathInInput {
		var resourceName string
		// check if file exists
		if _, err := os.Stat(param); err == nil {
			// Open file
			f, err := os.Open(param)
			if err != nil {
				return "", fmt.Errorf("Cannot open file: %s", err.Error())
			}

			defer f.Close()

			// read file metadata
			switch resourceType {
			case "right_scripts":
				metadata, err := ParseRightScriptMetadata(f)
				if err != nil {
					return "", err
				}
				resourceName = metadata.Name
			case "server_templates":
				metadata, err := ParseServerTemplate(f)
				if err != nil {
					return "", err
				}
				resourceName = metadata.Name
			default:
				return "", fmt.Errorf("Unknown resource")
			}

			if resourceName == "" {
				return "", fmt.Errorf("Failed to retrieve resource name from input file: %s", param)
			}
			param = resourceName
		} else {
			if !os.IsNotExist(err) {
				return "", fmt.Errorf("%s", err.Error())
			}
		}

	}
	hrefs, err = paramToHrefs(resourceType, param, revision)
	if err != nil {
		return "", err
	}
	revMessage := "and HEAD revision"
	if revision != 0 {
		revMessage = "and revision " + strconv.Itoa(revision)
	}

	if len(hrefs) > 1 {
		return "", fmt.Errorf("Matched multiple %ss with the name '%s' %s: %s."+
			"Don't know which one to use. Please delete one or specify an href to use.",
			resourceType, revMessage, strings.Join(hrefs, ", "), param)
	} else if len(hrefs) == 0 {
		return "", fmt.Errorf("Found no %s matching '%s' %s", resourceType, param, revMessage)
	} else {
		return hrefs[0], nil
	}
}

func getLink(links []map[string]string, name string) string {
	href := ""
	for _, l := range links {
		if l["rel"] == name {
			href = l["href"]
		}
	}
	return href
}

// Turn a mixed array of directories and files into a linear list of files
func walkPaths(paths []string) ([]string, error) {
	files := []string{}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return files, err
		}
		if info.IsDir() {
			err = filepath.Walk(path, func(p string, f os.FileInfo, err error) error {
				files = append(files, p)
				_, e := os.Stat(p)
				return e
			})
			if err != nil {
				return files, err
			}
		} else {
			files = append(files, path)
		}
	}
	return files, nil

}

func fatalError(format string, v ...interface{}) {
	msg := fmt.Sprintf("ERROR: "+format, v...)
	fmt.Fprintf(os.Stderr, "%s\n", msg)

	os.Exit(1)
}

func fmd5sum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return md5sum(file)
}

func md5sum(data io.Reader) (string, error) {
	hash := md5.New()

	_, err := io.Copy(hash, data)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Allows p{L} is a UTF8 equivalent of \w which will allow should allow for
// non ascii words
var disallowedFileChars = regexp.MustCompile(`[^\p{L}0-9_-]+`)

func cleanFileName(file string) string {
	s := disallowedFileChars.ReplaceAllString(file, "_") // KISS hopefully
	s = strings.Trim(s, "_")                             // axe trailing _ from ) type endings
	s = strings.Replace(s, "_-_", "-", -1)               // space dash space comes up quite a bit
	return s
}

func isDirectory(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fileInfo.IsDir()
}

// Finds a publication in the MultiCloud Marketplace.
// Params:
//   kind: one of RightScript, MultiCloudImage, ServerTemplate
//   name: name of publication to search for
//   revision: revision of the publication to search for. -1 means "latest"
//   matchers: Hash of string (field name) -> string (match value). additional matching criteria in case there are
//     multiple publications with the same name and revision. Usually `Publisher` is used as a tie breaker.
// Returns:
//   Publication if found. nil if not found. errors fatally if multiple publications are found.
func findPublication(kind string, name string, revision int, matchers map[string]string) (*cm15.Publication, error) {
	client, _ := Config.Account.Client15()

	pubLocator := client.PublicationLocator("/api/publications")

	if name == "" {
		return nil, fmt.Errorf("No Name given when looking up %s publication", kind)
	}

	if *debug {
		fmt.Printf("DEBUG: looking for publication with KIND:%s NAME:%s REVISION:%d MATCHERS:%v\n", kind, name, revision, matchers)
	}
	pubsUnfiltered, err := pubLocator.Index(rsapi.APIParams{"filter": []string{"name==" + name}})
	if err != nil {
		return nil, fmt.Errorf("Call to /api/publications failed: %s", err.Error())
	}
	maxRevision := -1
	var pubs []*cm15.Publication
	for _, pub := range pubsUnfiltered {
		// Recheck the name here, filter does a partial match and we need an exact one.
		// Also make sure the type is correct
		if pub.Name == name && kind == pub.ContentType {
			// We may provide additional matchers to break ties, i.e. the Description/Publisher field. In any are supplied
			// we make sure they all match.
			matched_all_matchers := true
			for fieldName, value := range matchers {
				v := reflect.Indirect(reflect.ValueOf(pub)).FieldByName(fieldName)
				if v.IsValid() {
					if v.String() != value {
						matched_all_matchers = false
					}
				}
			}
			if matched_all_matchers {
				// revision -1 means latest revision. We replace our list of found pubs with only the highest rev.
				if revision == -1 {
					if pub.Revision > maxRevision {
						maxRevision = pub.Revision
						pubs = []*cm15.Publication{pub}
					}
				} else if pub.Revision == revision {
					pubs = append(pubs, pub)
				}
			}
		}
	}

	if len(pubs) == 0 {
		return nil, nil
	} else if len(pubs) == 2 {
		fmt.Printf("Too many %s publications matching %s with revision %s\n", kind, name, formatRev(revision))
		for _, pub := range pubs {
			pubHref := getLink(pub.Links, "self")
			fmt.Printf("  Publisher:%s Revision:%d Href:%s\n", pub.Publisher, pub.Revision, pubHref)
		}
		return nil, fmt.Errorf("Too many publications")
	} else {
		return pubs[0], nil
	}
}

func getTagsByHref(href string) ([]string, error) {
	client, _ := Config.Account.Client15()
	var tags []string
	tagsLoc := client.TagLocator("/api/tags/by_resource")
	res, err := tagsLoc.ByResource([]string{href})
	if err != nil {
		return tags, err
	}
	if len(res) != 1 {
		return tags, fmt.Errorf("Could not find tags for href %s", href)
	}
	tagset := res[0]["tags"].([]interface{})
	for _, t := range tagset {
		th := t.(map[string]interface{})
		tags = append(tags, th["name"].(string))
	}

	return tags, nil
}

func setTagsByHref(href string, tags []string) error {
	client, _ := Config.Account.Client15()

	existingTags, err := getTagsByHref(href)
	if err != nil {
		return err
	}
	toDelete := []string{}
	for _, et := range existingTags {
		seen := false
		for _, t := range tags {
			if t == et {
				seen = true
			}
		}
		if !seen {
			toDelete = append(toDelete, et)
		}
	}
	if len(toDelete) > 0 {
		tagsLoc := client.TagLocator("/api/tags/multi_delete")
		err = tagsLoc.MultiDelete([]string{href}, toDelete)
		if err != nil {
			return err
		}
	}

	if len(tags) > 0 {
		tagsLoc := client.TagLocator("/api/tags/multi_add")
		return tagsLoc.MultiAdd([]string{href}, tags)
	}
	return nil
}

// Carriage returns get inserted by the API and mess up formatting of the YAML file.
func removeCarriageReturns(s string) string {
	return strings.Replace(s, "\r", "", -1)
}

func formatRev(rev int) string {
	if rev == -1 {
		return "latest"
	} else if rev == 0 {
		return "head"
	} else {
		return fmt.Sprintf("%d", rev)
	}
}
