package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mattn/go-colorable"
	"github.com/tonnerre/golang-pretty"

	"github.com/alecthomas/kingpin"
	"gopkg.in/inconshreveable/log15.v2"

	"github.com/rightscale/rsc/cm15"
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
	rightScriptListFilter = rightScriptList.Flag("filter", "Filter by name").Short('f').Required().String()

	rightScriptUpload      = rightScript.Command("upload", "Upload a RightScript")
	rightScriptUploadFile  = rightScriptUpload.Arg("file", "File to upload").Required().String()
	rightScriptUploadForce = rightScriptUpload.Flag("force", "Force upload of file if metadata is not present").Bool()

	rightScriptDownload     = rightScript.Command("download", "Download a RightScript to a file or files")
	rightScriptDownloadName = rightScriptDownload.Flag("name", "Script Name").Short('s').String()
	rightScriptDownloadId   = rightScriptDownload.Flag("id", "Script ID").Short('i').Int()

	rightScriptMetadata     = rightScript.Command("metadata", "Add RightScript YAML metadata comments to a file or files")
	rightScriptMetadataFile = rightScriptMetadata.Flag("file", "File or directory to set metadata for").Short('f').String()

	rightScriptValidate      = rightScript.Command("validate", "Validate RightScript YAML metadata comments in a file or files")
	rightScriptValidatePaths = rightScriptValidate.Arg("path", "Path to script file or directory containing script files").Required().ExistingFilesOrDirs()
)

func main() {
	command := kingpin.MustParse(app.Parse(os.Args[1:]))

	err := readConfig(*configFile, *environment)
	client := config.environment.Client15()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: Error reading config file: %s\n", filepath.Base(os.Args[0]), err)
		os.Exit(1)
	}

	// Handle logginng
	handler := log15.StreamHandler(colorable.NewColorableStdout(), log15.TerminalFormat())
	log15.Root().SetHandler(handler)
	if *debug {
		log.Logger.SetHandler(handler)
	}
	app.Writer(os.Stdout)

	switch command {
	case rightScriptList.FullCommand():
		rightscriptLocator := client.RightScriptLocator("/api/right_scripts")
		var apiParams = rsapi.APIParams{"filter": []string{"name==" + *rightScriptListFilter}}
		fmt.Printf("LIST %s:\n", *rightScriptListFilter)
		rightscripts, err := rightscriptLocator.Index(
			apiParams,
		)
		if err != nil {
			fatalError("%#v", err)
		}
		for _, rs := range rightscripts {
			fmt.Printf("/api/right_scripts/%s %s\n", rs.Id, rs.Name)
		}
	case rightScriptUpload.FullCommand():
		// fmt.Println(*rightScriptUpload)
		fmt.Printf("Uploading %s:", *rightScriptUploadFile)
		f, err := os.Open(*rightScriptUploadFile)
		if err != nil {
			fatalError("Cannot open %s", *rightScriptUploadFile)
		}
		defer f.Close()
		metadata, err := ParseRightScriptMetadata(f)
		fmt.Printf("%v -- %v\n", metadata, err)

		if err != nil {
			if !*rightScriptUploadForce {
				fatalError("No metadata for %s. Use --force to upload anyways.", *rightScriptUploadFile)
			}
		}

		fmt.Println("Testing")

		err = pushToRightscale(*rightScriptUploadFile, metadata)
		fmt.Println(err)
	case rightScriptDownload.FullCommand():
		fmt.Println(*rightScriptDownload)
	case rightScriptMetadata.FullCommand():
		fmt.Println(*rightScriptMetadata)
	case rightScriptValidate.FullCommand():
		for _, path := range *rightScriptValidatePaths {
			info, err := os.Stat(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Base(os.Args[0]), err)
				os.Exit(1)
			}
			if info.IsDir() {
				// TODO: recurse?
			} else {
				err = validateRightScript(path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Base(os.Args[0]), err)
					os.Exit(1)
				}
			}
		}
	}
}

// type RightScript struct {
// 	Href string
// 	Path string
//  Metadata *RightScriptMetadata
// }
// func (*RightScript) pushToRightscale() {
func pushToRightscale(file string, metadata *RightScriptMetadata) error {
	client := config.environment.Client15()
	rightscriptLocator := client.RightScriptLocator("/api/right_scripts")
	apiParams := rsapi.APIParams{"filter": []string{"name==" + metadata.Name}}
	rightscripts, err := rightscriptLocator.Index(apiParams)
	if err != nil {
		return err
	}
	foundId := ""
	for _, rs := range rightscripts {
		fmt.Printf("%#v\n", rs)
		// Recheck the name here, filter does a impartial match and we need an exact one
		if rs.Name == metadata.Name && rs.Revision == 0 {
			if foundId != "" {
				fatalError("Error, matched multiple RightScripts with the same name, please delete one: %d %d", rs.Id, foundId)
			} else {
				foundId = rs.Id
			}
		}
	}

	fmt.Printf("Here: %d\n", foundId)

	fileSrc, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	if foundId == "" {
		fmt.Printf("Creating a new RightScript named '%s' from %s\n", metadata.Name, file)
		// New one, perform create call
		params := cm15.RightScriptParam2{
			Name:        metadata.Name,
			Description: metadata.Description,
			Source:      string(fileSrc),
		}
		//rightscriptLocator = client.RightScriptLocator(fmt.Sprintf("/api/right_scripts", foundId))
		locator, err := rightscriptLocator.Create(&params)
		fmt.Println(locator, err)
		return err
	} else {
		// apiParams = rsapi.APIParams{
		// 	"Name":        metadata.Name,
		// 	"Description": metadata.Description,
		// 	"Source":      string(fileSrc),
		// }
		params := cm15.RightScriptParam3{
			Name:        metadata.Name,
			Description: metadata.Description,
			Source:      string(fileSrc),
		}
		rightscriptLocator = client.RightScriptLocator(fmt.Sprintf("/api/right_scripts/%s", foundId))
		err = rightscriptLocator.Update(&params)
		fmt.Println(err)
		return err
		// Found existing, do an update
	}
	return nil
}

func fatalError(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v)
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
	pretty.Println(metadata)

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
