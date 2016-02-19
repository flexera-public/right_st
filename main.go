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
	"regexp"
	"strconv"
	"strings"

	"github.com/alecthomas/kingpin"
	"github.com/mattn/go-colorable"
	"github.com/rightscale/rsc/httpclient"
	"github.com/rightscale/rsc/log"
	"github.com/rightscale/rsc/rsapi"
	"gopkg.in/inconshreveable/log15.v2"
)

var (
	app         = kingpin.New("right_st", "A command-line application for managing RightScripts")
	debug       = app.Flag("debug", "Debug mode").Short('d').Bool()
	configFile  = app.Flag("config", "Set the config file path.").Short('c').Default(defaultConfigFile()).String()
	environment = app.Flag("environment", "Set the RightScale login environment.").Short('e').String()

	// ----- ServerTemplates -----
	stCmd = app.Command("st", "ServerTemplate")

	stShowCmd        = stCmd.Command("show", "Show a single ServerTemplate")
	stShowNameOrHref = stShowCmd.Arg("name|href|id", "ServerTemplate Name or HREF or Id").Required().String()

	stUploadCmd   = stCmd.Command("upload", "Upload a ServerTemplate specified by a YAML document")
	stUploadPaths = stUploadCmd.Arg("path", "File or directory containing script files to upload").Required().ExistingFilesOrDirs()

	stDownloadCmd        = stCmd.Command("download", "Download a ServerTemplate and all associated RightScripts/Attachments to disk")
	stDownloadNameOrHref = stDownloadCmd.Arg("name|href|id", "Script Name or HREF or Id").Required().String()
	stDownloadTo         = stDownloadCmd.Arg("path", "Download location").String()

	stValidateCmd   = stCmd.Command("validate", "Validate a ServerTemplate YAML document")
	stValidatePaths = stValidateCmd.Arg("path", "Path to script file(s)").Required().ExistingFiles()

	// ----- RightScripts -----
	rightScriptCmd = app.Command("rightscript", "RightScript")

	rightScriptShowCmd        = rightScriptCmd.Command("show", "Show a single RightScript and its attachments")
	rightScriptShowNameOrHref = rightScriptShowCmd.Arg("name|href|id", "Script Name or HREF or Id").Required().String()

	rightScriptUploadCmd   = rightScriptCmd.Command("upload", "Upload a RightScript")
	rightScriptUploadPaths = rightScriptUploadCmd.Arg("path", "File or directory containing script files to upload").Required().ExistingFilesOrDirs()
	rightScriptUploadForce = rightScriptUploadCmd.Flag("force", "Force upload of file if metadata is not present").Short('f').Bool()

	rightScriptDownloadCmd        = rightScriptCmd.Command("download", "Download a RightScript to a file or files")
	rightScriptDownloadNameOrHref = rightScriptDownloadCmd.Arg("name|href|id", "Script Name or HREF or Id").Required().String()
	rightScriptDownloadTo         = rightScriptDownloadCmd.Arg("path", "Download location").String()

	rightScriptScaffoldCmd      = rightScriptCmd.Command("scaffold", "Add RightScript YAML metadata comments to a file or files")
	rightScriptScaffoldPaths    = rightScriptScaffoldCmd.Arg("path", "File or directory to set metadata for").Required().ExistingFilesOrDirs()
	rightScriptScaffoldNoBackup = rightScriptScaffoldCmd.Flag("no-backup", "Do not create backup files before scaffolding").Short('n').Bool()

	rightScriptValidateCmd   = rightScriptCmd.Command("validate", "Validate RightScript YAML metadata comments in a file or files")
	rightScriptValidatePaths = rightScriptValidateCmd.Arg("path", "Path to script file or directory containing script files").Required().ExistingFilesOrDirs()

	// ----- Configuration -----
	configCmd = app.Command("config", "Manage Configuration")

	configUpdateCmd = configCmd.Command("update", "Add/Edit configuration environment")
	configUpdateEnv = configUpdateCmd.Arg("environment", "Environment to manage in config file").Required().String()

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

	err := readConfig(*configFile, *environment)
	if err != nil && !strings.HasPrefix(command, "config") {
		fatalError("%s: Error reading config file: %s\n", filepath.Base(os.Args[0]), err.Error())
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

	if config.GetBool("update.check") && !strings.HasPrefix(command, "update") {
		defer UpdateCheck(VV, os.Stderr)
	}

	switch command {
	case stShowCmd.FullCommand():
		href, err := paramToHref("server_templates", *stShowNameOrHref, 0)
		if err != nil {
			fatalError("%s", err.Error())
		}
		stShow(href)
	case stUploadCmd.FullCommand():
		files, err := walkPaths(*stUploadPaths)
		if err != nil {
			fatalError("%s\n", err.Error())
		}
		stUpload(files)
	case stDownloadCmd.FullCommand():
		href, err := paramToHref("server_templates", *stDownloadNameOrHref, 0)
		if err != nil {
			fatalError("%s", err.Error())
		}
		stDownload(href, *stDownloadTo)
	case stValidateCmd.FullCommand():
		files, err := walkPaths(*stValidatePaths)
		if err != nil {
			fatalError("%s\n", err.Error())
		}
		stValidate(files)
	case rightScriptShowCmd.FullCommand():
		href, err := paramToHref("right_scripts", *rightScriptShowNameOrHref, 0)
		if err != nil {
			fatalError("%s", err.Error())
		}
		rightScriptShow(href)
	case rightScriptUploadCmd.FullCommand():
		rightScriptUpload(*rightScriptUploadPaths, *rightScriptUploadForce)
	case rightScriptDownloadCmd.FullCommand():
		href, err := paramToHref("right_scripts", *rightScriptDownloadNameOrHref, 0)
		if err != nil {
			fatalError("%s", err.Error())
		}
		rightScriptDownload(href, *rightScriptDownloadTo)
	case rightScriptScaffoldCmd.FullCommand():
		files, err := walkPaths(*rightScriptScaffoldPaths)
		if err != nil {
			fatalError("%s\n", err.Error())
		}
		rightScriptScaffold(files, !*rightScriptScaffoldNoBackup)
	case rightScriptValidateCmd.FullCommand():
		files, err := walkPaths(*rightScriptValidatePaths)
		if err != nil {
			fatalError("%s\n", err.Error())
		}
		rightScriptValidate(files)
	case configUpdateCmd.FullCommand():
		generateConfig(*configFile, *configUpdateEnv)
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

func paramToHref(resourceType, param string, revision int) (string, error) {
	client := config.environment.Client15()

	idMatch := regexp.MustCompile(`^\d+$`)
	hrefMatch := regexp.MustCompile(fmt.Sprintf("^/api/%s/\\d+$", resourceType))

	var href string
	if idMatch.Match([]byte(param)) {
		href = fmt.Sprintf("/api/%s/%s", resourceType, param)
	} else if hrefMatch.Match([]byte(param)) {
		href = param
	} else {
		payload := rsapi.APIParams{}
		params := rsapi.APIParams{"filter[]": []string{"name==" + param}}
		uriPath := fmt.Sprintf("/api/%s", resourceType)

		req, err := client.BuildHTTPRequest("GET", uriPath, "1.5", params, payload)
		if err != nil {
			return "", err
		}
		resp, err := client.PerformRequest(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		respBody, _ := ioutil.ReadAll(resp.Body)
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return "", fmt.Errorf("invalid response %s: %s", resp.Status, string(respBody))
		}
		items := []Iterable{}
		err = json.Unmarshal(respBody, &items)
		if err != nil {
			return "", err
		}
		count := 0
		for _, item := range items {
			if item.Name == param && item.Revision == revision {
				href = getLink(item.Links, "self")
				count = count + 1
			}
		}
		if count == 0 {
			return "", fmt.Errorf("Found no %s matching '%s'", resourceType, param)
		} else if count > 1 {
			revMessage := " and HEAD revision. "
			if revision != 0 {
				revMessage = " and revision " + strconv.Itoa(revision) + ". "
			}
			return "", fmt.Errorf("Matched multiple %s with the name %s"+revMessage+
				"Don't know which one to use. Please delete one or specify an HREF to use such as %s", resourceType, param, href)
		}
	}
	return href, nil
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
