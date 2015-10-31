package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mattn/go-colorable"
	"github.com/tonnerre/golang-pretty"

	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/inconshreveable/log15.v2"
	"gopkg.in/rightscale/rsc.v4/log"
)

var (
	app         = kingpin.New("right_st", "RightScale ServerTemplate and RightScript tool")
	configFile  = app.Flag("config", "Set the config file path.").Short('c').Default(defaultConfigFile()).String()
	environment = app.Flag("environment", "Set the RightScale login environment.").Short('e').String()
	account     = app.Flag("account", "Set the RightScale account ID.").Short('a').Int()
	host        = app.Flag("host", "RightScale login endpoint (e.g. 'us-3.rightscale.com')").Short('h').String()

	rightScript = app.Command("rightscript", "RightScript stuff")

	rightScriptList     = rightScript.Command("list", "List RightScripts")
	rightScriptUpload   = rightScript.Command("upload", "Upload a RightScript")
	rightScriptDownload = rightScript.Command("download", "Download a RightScript to a file or files")
	rightScriptMetadata = rightScript.Command("metadata", "Add RightScript YAML metadata comments to a file or files")

	rightScriptValidate      = rightScript.Command("validate", "Validate RightScript YAML metadata comments in a file or files")
	rightScriptValidatePaths = rightScriptValidate.Arg("path", "Path to script file or directory containing script files").Required().ExistingFilesOrDirs()
)

func main() {
	handler := log15.StreamHandler(colorable.NewColorableStdout(), log15.TerminalFormat())
	log15.Root().SetHandler(handler)
	log.Logger.SetHandler(handler)

	app.Writer(os.Stdout)

	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case rightScriptList.FullCommand():
		fmt.Println(*rightScriptList)
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
				validateRightScript(path)
			}
		}
	}

	fmt.Println(*configFile, *environment, *account, *host, *rightScript)
}

func validateRightScript(path string) {
	script, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Base(os.Args[0]), err)
		os.Exit(1)
	}
	defer script.Close()
	pretty.Println(ParseRightScriptMetadata(script))
}
