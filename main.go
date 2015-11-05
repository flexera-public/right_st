package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mattn/go-colorable"
	"github.com/tonnerre/golang-pretty"

	"github.com/alecthomas/kingpin"
	"gopkg.in/inconshreveable/log15.v2"

	"github.com/rightscale/rsc/cm15"
	"github.com/rightscale/rsc/httpclient"
	"github.com/rightscale/rsc/log"
	"github.com/rightscale/rsc/rsapi"
)

var (
	app         = kingpin.New("right_st", "A command-line application for managing RightScripts")
	version     = app.Flag("version", "Print version").Short('v').Bool()
	debug       = app.Flag("debug", "Debug mode").Short('d').Bool()
	configFile  = app.Flag("config", "Set the config file path.").Short('c').Default(defaultConfigFile()).String()
	environment = app.Flag("environment", "Set the RightScale login environment.").Short('e').String()

	rightScript = app.Command("rightscript", "RightScript stuff")

	rightScriptList       = rightScript.Command("list", "List RightScripts")
	rightScriptListFilter = rightScriptList.Arg("filter", "Filter by name").Required().String()

	rightScriptUpload      = rightScript.Command("upload", "Upload a RightScript")
	rightScriptUploadPaths = rightScriptUpload.Arg("path", "File or directory containing script files to upload").Required().ExistingFilesOrDirs()
	rightScriptUploadForce = rightScriptUpload.Flag("force", "Force upload of file if metadata is not present").Bool()

	rightScriptDownload           = rightScript.Command("download", "Download a RightScript to a file or files")
	rightScriptDownloadNameOrHref = rightScriptDownload.Arg("name_or_href", "Script Name or Href").Required().String()
	rightScriptDownloadTo         = rightScriptDownload.Arg("path", "Download location").String()

	rightScriptScaffold      = rightScript.Command("scaffold", "Add RightScript YAML metadata comments to a file or files")
	rightScriptScaffoldPaths = rightScriptScaffold.Arg("path", "File or directory to set metadata for").Required().ExistingFilesOrDirs()

	rightScriptValidate      = rightScript.Command("validate", "Validate RightScript YAML metadata comments in a file or files")
	rightScriptValidatePaths = rightScriptValidate.Arg("path", "Path to script file or directory containing script files").Required().ExistingFilesOrDirs()
)

func main() {
	app.HelpFlag.Short('h')
	command := kingpin.MustParse(app.Parse(os.Args[1:]))

	err := readConfig(*configFile, *environment)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: Error reading config file: %s\n", filepath.Base(os.Args[0]), err)
		os.Exit(1)
	}
	client := config.environment.Client15()

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
	app.Writer(os.Stdout)

	switch command {
	case rightScriptList.FullCommand():
		rightscriptLocator := client.RightScriptLocator("/api/right_scripts")
		var apiParams = rsapi.APIParams{"filter": []string{"name==" + *rightScriptListFilter}}
		fmt.Printf("Listing %s:\n", *rightScriptListFilter)
		//log15.Info("Listing", "RightScript", *rightScriptListFilter)
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
	case rightScriptUpload.FullCommand():
		// Pass 1, perform validations, gather up results
		scripts := []RightScript{}
		paths, err := walkPaths(rightScriptUploadPaths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Base(os.Args[0]), err)
			os.Exit(1)
		}

		for _, p := range paths {
			fmt.Printf("Uploading %s\n", p)
			f, err := os.Open(p)
			if err != nil {
				fatalError("Cannot open %s", p)
			}
			defer f.Close()
			metadata, err := ParseRightScriptMetadata(f)

			if metadata.Name == "" {
				if !*rightScriptUploadForce {
					fatalError("No embedded metadata for %s. Use --force to upload anyways.", p)
				}
				scriptname := path.Base(p)
				scriptext := path.Ext(scriptname)
				scriptname = strings.TrimRight(scriptname, scriptext)
				metadata.Name = scriptname
			}

			script := RightScript{"", p, metadata}
			scripts = append(scripts, script)
		}

		// Pass 2, upload
		for _, script := range scripts {
			err = script.Push()
			if err != nil {
				fatalError("%s", err.Error())
			}
		}
	case rightScriptDownload.FullCommand():
		rsIdMatch := regexp.MustCompile(`^\d+$`)
		rsHrefMatch := regexp.MustCompile(`^/api/right_scripts/\d+$`)

		var href string

		if rsIdMatch.Match([]byte(*rightScriptDownloadNameOrHref)) {
			href = fmt.Sprintf("/api/right_scripts/%s", *rightScriptDownloadNameOrHref)
		} else if rsHrefMatch.Match([]byte(*rightScriptDownloadNameOrHref)) {
			href = *rightScriptDownloadNameOrHref
		} else {
			rightscriptLocator := client.RightScriptLocator("/api/right_scripts")
			apiParams := rsapi.APIParams{"filter": []string{"name==" + *rightScriptDownloadNameOrHref}}
			rightscripts, err := rightscriptLocator.Index(apiParams)
			if err != nil {
				fatalError("%s", err.Error())
			}
			foundId := ""
			for _, rs := range rightscripts {
				//fmt.Printf("%#v\n", rs)
				// Recheck the name here, filter does a impartial match and we need an exact one
				// TODO, do first pass for head revisions only, second for non-heads?
				if rs.Name == *rightScriptDownloadNameOrHref && rs.Revision == 0 {
					if foundId != "" {
						fatalError("Error, matched multiple RightScripts with the same name. Don't know which one to download. Please delete one or specify an HREF to download such as /api/right_scripts/%d", rs.Id)
					} else {
						foundId = rs.Id
					}
				}
			}
			if foundId == "" {
				fatalError("Found no RightScripts matching %s", *rightScriptDownloadNameOrHref)
			}
			href = fmt.Sprintf("/api/right_scripts/%s", foundId)
		}

		rightscriptLocator := client.RightScriptLocator(href)
		// attachmentsLocator := client.RightScriptLocator(fmt.Sprintf("%s/attachments", href))
		// sourceLocator := client.RightScriptLocator(fmt.Sprintf("%s/source", href))

		rightscript, err := rightscriptLocator.Show()
		if err != nil {
			fatalError("Could not find rightscript with href %s: %s", href, err.Error())
		}
		source, err := getSource(rightscriptLocator)
		if err != nil {
			fatalError("Could get soruce for rightscript with href %s: %s", href, err.Error())
		}

		// attachments, err2 := attachmentsLocator.Index(rsapi.APIParams{})
		if *rightScriptDownloadTo == "" {
			*rightScriptDownloadTo = rightscript.Name
		}
		fmt.Printf("Downloading '%s' to %s\n", rightscript.Name, *rightScriptDownloadTo)
		err = ioutil.WriteFile(*rightScriptDownloadTo, source, 0755)
		if err != nil {
			fatalError("Could not create file: %s", err.Error())
		}

	case rightScriptScaffold.FullCommand():
		paths, err := walkPaths(rightScriptScaffoldPaths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Base(os.Args[0]), err)
			os.Exit(1)
		}

		for _, path := range paths {
			err = AddRightScriptMetadata(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Base(os.Args[0]), err)
				os.Exit(1)
			}
		}
	case rightScriptValidate.FullCommand():
		paths, err := walkPaths(rightScriptValidatePaths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Base(os.Args[0]), err)
			os.Exit(1)
		}

		err_encountered := false

		for _, path := range paths {
			err = validateRightScript(path)
			if err != nil {
				err_encountered = true
				fmt.Fprintf(os.Stderr, "%s - %s: %s\n", path, filepath.Base(os.Args[0]), err)
			}
		}
		if err_encountered {
			os.Exit(1)
		}
	}
}

// Turn a mixed array of directories and files into a linear list of files
func walkPaths(paths *[]string) ([]string, error) {
	files := []string{}
	for _, path := range *paths {
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

type RightScript struct {
	Href     string
	Path     string
	Metadata *RightScriptMetadata
}

func (r *RightScript) Push() error {
	client := config.environment.Client15()
	rightscriptLocator := client.RightScriptLocator("/api/right_scripts")
	apiParams := rsapi.APIParams{"filter": []string{"name==" + r.Metadata.Name}}
	rightscripts, err := rightscriptLocator.Index(apiParams)
	if err != nil {
		return err
	}
	foundId := ""
	for _, rs := range rightscripts {
		//fmt.Printf("%#v\n", rs)
		// Recheck the name here, filter does a impartial match and we need an exact one
		if rs.Name == r.Metadata.Name && rs.Revision == 0 {
			if foundId != "" {
				fatalError("Error, matched multiple RightScripts with the same name, please delete one: %d %d", rs.Id, foundId)
			} else {
				foundId = rs.Id
			}
		}
	}

	pathSrc, err := ioutil.ReadFile(r.Path)
	if err != nil {
		return err
	}

	if foundId == "" {
		fmt.Printf("Creating a new RightScript named '%s' from %s\n", r.Metadata.Name, r.Path)
		// New one, perform create call
		params := cm15.RightScriptParam2{
			Name:        r.Metadata.Name,
			Description: r.Metadata.Description,
			Source:      string(pathSrc),
		}
		//rightscriptLocator = client.RightScriptLocator(fmt.Sprintf("/api/right_scripts", foundId))
		_, err = rightscriptLocator.Create(&params)
	} else {
		fmt.Printf("Updating existing RightScript named '%s' from %s\n", r.Metadata.Name, r.Path)

		params := cm15.RightScriptParam3{
			Name:        r.Metadata.Name,
			Description: r.Metadata.Description,
			Source:      string(pathSrc),
		}
		rightscriptLocator = client.RightScriptLocator(fmt.Sprintf("/api/right_scripts/%s", foundId))
		err = rightscriptLocator.Update(&params)
		// Found existing, do an update
	}
	return err
}

func fatalError(format string, v ...interface{}) {
	msg := fmt.Sprintf("ERROR: "+format, v)
	fmt.Println(msg)
	os.Exit(1)
}

func validateRightScript(path string) error {
	script, err := os.Open(path)
	if err != nil {
		return err
	}
	defer script.Close()

	metadata, err := ParseRightScriptMetadata(script)
	if err != nil {
		return err
	}
	if *debug {
		pretty.Println(metadata)
	}
	fmt.Printf("%s - valid metadata\n", path)

	for _, attachment := range metadata.Attachments {
		md5, err := md5Attachment(path, attachment)
		if err != nil {
			return err
		}
		fmt.Println(attachment, md5)
	}

	return nil
}

func md5Attachment(script, attachment string) (string, error) {
	path := filepath.Join(filepath.Dir(script), attachment)
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := md5.New()

	_, err = io.Copy(hash, file)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
