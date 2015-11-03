package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mattn/go-colorable"
	"github.com/tonnerre/golang-pretty"

	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/inconshreveable/log15.v2"

	"gopkg.in/rightscale/rsc.v4/log"
	"gopkg.in/rightscale/rsc.v4/rsapi"
)

var (
	app         = kingpin.New("right_st", "A command-line application for managing RightScripts")
	version     = app.Flag("version", "Print version").Short('v').Bool()
	debug       = app.Flag("debug", "Debug mode").Short('d').Bool()
	configFile  = app.Flag("config", "Set the config file path.").Short('c').Default(defaultConfigFile()).String()
	environment = app.Flag("environment", "Set the RightScale login environment.").Short('e').String()

	rightScript = app.Command("rightscript", "RightScript stuff")

	rightScriptList       = rightScript.Command("list", "List RightScripts")
	rightScriptListFilter = rightScriptList.Flag("filter", "Filter by name").Required().String()

	rightScriptUpload     = rightScript.Command("upload", "Upload a RightScript")
	rightScriptUploadFile = rightScriptUpload.Flag("file", "File or directory to upload").Short('f').String()

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
	log.Logger.SetHandler(handler)
	app.Writer(os.Stdout)

	switch command {
	case rightScriptList.FullCommand():
		rightscriptLocator := client.RightScriptLocator("/api/right_scripts")
		var apiParams = rsapi.APIParams{"filter": []string{"name==" + *rightScriptListFilter}}
		fmt.Printf("LIST %s", *rightScriptListFilter)
		rightscripts, err := rightscriptLocator.Index(
			apiParams,
		)
		if err != nil {
			fatalError("%#v", err)
		}
		for _, rs := range rightscripts {
			fmt.Printf("%s\n", rs.Name)
		}
	case rightScriptUpload.FullCommand():
		fmt.Println(*rightScriptUpload)
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

	fmt.Println("Start - options:", *configFile, config, *rightScript)

	fmt.Println("Done -- authenticated")
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
