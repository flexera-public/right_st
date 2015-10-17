package main

import (
	"fmt"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"
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
)

func main() {
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
	}

	fmt.Println(*configFile, *environment, *account, *host, *rightScript)
}
