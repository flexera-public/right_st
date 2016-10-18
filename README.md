# right_st

`right_st` is a tool for managing RightScale ServerTemplate and RightScripts. The tool is able to download, upload, and show ServerTemplate and RightScripts using RightScale's API. This tool can easily be hooked into Travis CI or other build systems to manage these design objects if stored in Github. See below for usage examples.

[![Travis CI Build Status](https://travis-ci.org/rightscale/right_st.svg?branch=master)](https://travis-ci.org/rightscale/right_st?branch=master)
[![AppVeyor Build Status](https://ci.appveyor.com/api/projects/status/github/rightscale/right_st?branch=master&svg=true)](https://ci.appveyor.com/project/RightScale/right-st?branch=master)

* [Installation](#installation)
  * [Configuration](#configuration)
* [Managing RightScripts](#managing-rightscripts)
  * [RightScript Usage](#rightscript-usage)
* [Managing ServerTemplates](#managing-servertemplates)
  * [ServerTemplate Usage](#servertemplate-usage)
* [Contributors](#contributors)
* [License](#license)

## Installation

Since `right_st` is written in Go it is compiled to a single static binary. Extract and run the executable below:

* Linux: [v1/right_st-linux-amd64.tgz](https://binaries.rightscale.com/rsbin/right_st/v1/right_st-linux-amd64.tgz)
* Mac OS X: [v1/right_st-darwin-amd64.tgz](https://binaries.rightscale.com/rsbin/right_st/v1/right_st-darwin-amd64.tgz)
* Windows: [v1/right_st-windows-amd64.zip](https://binaries.rightscale.com/rsbin/right_st/v1/right_st-windows-amd64.zip)

### Configuration

Right ST interfaces with the [RightScale API](http://reference.rightscale.com/api1.5). Credentials for the API can be provided in two ways:

1. YAML-based configuration file -  Run `right_st config account <name>`, where name is a nickname for the account, to interactively write the configuration file into `$HOME/.right_st.yml` for the first time. You will be prompted for the following fields:
    * Account ID - Numeric account number, such as `60073`
    * API endpoint host - Hostname, typically `my.rightscale.com`
    * Refresh Token - Your personal OAuth token available from **Settings > Account Settings > API Credentials** in the RightScale Cloud Management dashboard
2. Environment variables - These are meant to be used by build systems such as Travis CI. The following vars must be set: `RIGHT_ST_LOGIN_ACCOUNT_ID`, `RIGHT_ST_LOGIN_ACCOUNT_HOST`, `RIGHT_ST_LOGIN_ACCOUNT_REFRESH_TOKEN`. These variables are equivalent to the ones described in the YAML section above.

## Managing RightScripts

RightScripts consist of a script body, attachments, and metadata. Metadata is embedded in the script as a comment between the hashbang and script body in the [RightScript Metadata Comments](http://docs.rightscale.com/cm/dashboard/design/rightscripts/rightscripts_metadata_comments.html) format. This allows a single script file to be a fully self-contained respresentation of a RightScript. Metadata comment format is as follows:

| Field | Format | Description |
| ----- | ------ | ----------- |
| RightScript Name | String | Name of RightScript. Name must be unique for your account. |
| Description | String | Description field for the RightScript. Free form text which can be Markdown |
| Inputs | Hash of String -> Input | The hash key is the input name. The hash value is an Input definition (defined below) |
| Attachments | Array of Strings | Each string is a filename of an attachment file. Relative or absolute paths supported. Relative paths will be placed in an "attachments/" subdirectory. For example "1/foo" will expect a file foo at "attachments/1/foo"|

Input definition format is as follows:

| Field | Format | Description |
| ----- | ------ | ----------- |
| Category | String | Category to group Input under |
| Description | String | Description field for the Input |
| Input Type | String | `single` or `array` |
| Default | String | Default value. Value must be in [Inputs 2.0 format](http://reference.rightscale.com/api1.5/resources/ResourceInputs.html) from RightScale API which consists of a type followed by a colon then the value. I.e. "text:Foo", "ignore", "env:ENV_VAR" |
| Required | Boolean | `true` or `false`. Whether or not the Input is required |
| Advanced | Boolean | `true` or `false`. Whether or not the Input is advanced (hidden by default) |
| Possible Values | Array of Strings | If supplied, a drop down list of values will be supplied in the UI for this input. Each string must be a text type in [Inputs 2.0 format](http://reference.rightscale.com/api1.5/resources/ResourceInputs.html). |


Example RightScript is as follows. This RightScript has one attachment, which must be located at "attachments/foo" relative
to the script.

```bash
#! /bin/bash -e

# ---
# RightScript Name: Run Foo Tool
# Description: Runs attached foo executable with input
# Inputs:
#   FOO_PARAM:
#     Category: RightScale
#     Description: A parameter to the foo tool
#     Input Type: single
#     Required: false
#     Advanced: true
#     Default: "text:foo1"
#     Possible Values: ["text:foo1", "text:foo2"]
# Attachments:
# - foo
# ...
#

cp -f $RS_ATTACH_DIR/foo /usr/local/bin/foo
chmod a+x /usr/local/bin/foo
foo $FOO_PARAM
```

### RightScript Usage
The following RightScript related commands are supported:

```
right_st rightscript show <name|href|id>
  Show a single RightScript and its attachments. 

right_st rightscript upload [<flags>] <path>...
  Upload a RightScript
  Flags:
    -f, --force: Force upload of RightScript despite lack of Metadata comments
    -x, --prefix: Append a prefix to RightScript's name when uploading. For 
                  creating dev/test versions of scripts.

right_st rightscript download <name|href|id> [<path>]
  Download a RightScript to a file. Metadata comments will automatically be 
   inserted into RightScripts that don't have it.

right_st rightscript scaffold [<flags>] <path>...
  Add RightScript YAML metadata comments to a file or files
  Flags:
    -f, --force: Force regeneration of scaffold data.

right_st rightscript validate <path>...
  Validate RightScript YAML metadata comments in a file or files
```


## Managing ServerTemplates

ServerTemplates are defined by a YAML format representing the ServerTemplate. The following keys are supported:

| Field | Format | Description |
| ----- | ------ | ----------- |
| Name | String | Name of the ServerTemplate. Name must be unique for your account. |
| Description | String | Description field for the ServerTemplate. |
| RightScripts | Hash of String -> Array of RightScripts| The hash key is the sequence type, one of "Boot", "Operational", or "Decommission". The hash value is a array of RightScripts. Each RightScript can be specified in one of three ways, as a 1. "local" managed 2. "published", or 3. "external" RightScript. A locally managed RightScript is specified as a pathname to a file on disk and the file contents are synchonized to the HEAD revision of a RightScript in your local account. Published RightScripts are links to pre-existing RightScripts shared in the MultiCloud marketplace and consist of a hash specifying a Name/Revision/Publisher to look up. External RightScripts are pre-existing RightScripts consisting of a Name/Revision pair and will not search the MultiCloud marketplace. |
| Inputs | Hash of String -> String | The hash key is the input name. The hash value is the default value. Note this inputs array is much simpler than the Input definition in RightScripts - only default values can be overridden in a ServerTemplate. |
| MultiCloudImages | Array of MultiCloudImages | An array of MultiCloudImage definitions. A MultiCloudImage definition is a hash of fields taking a few different formats. See section below for further details. |
| Alerts | Array of Alerts | An array of Alert definitions, defined below. |

A MultiCloudImage definition allows you to specify an MCI in four different ways by supplying different hash keys. The first three combinations specified below allow you to use pre-existing MCIs. The fourth one allows you to fully manage an MCI in your local account:

1. 'Name' and 'Revision' and 'Publisher': Name/Revision/Publisher of MCI available in the MultiCloud Marketplace. The MCI will be automatically imported into the account if it's not already there. Preferred. "latest" may be specified for the revision to get the latest revision.
2. 'Name' and 'Revision'.: Name/Revision of MCI in your local account. It will not attempt to be autoimported from the MultiCloud Marketplace. "latest" or "head" may be specified to get the latest committed revision and "head" revision respectively.
3. 'Href': Href of the MCI in the account being uploaded. This is a fallback in case 1 or 2 doesn't work.
4. Fully specified. The following keys are supported:
    * 'Name' - String - Name of the MCI
    * 'Tags' - Array of Strings - Representing tags on the MCI. Typically 'rs_agent:type=right_link_lite' will be required.
    * 'Description' - String - Optional description for the MCI
    * 'Settings' - Array of Settings - A setting represents the following API resource: [MultiCloudImageSettings](http://reference.rightscale.com/api1.5/resources/ResourceMultiCloudImageSettings.html). The following keys are used:
        * `Cloud` - String - Required - Name of cloud
        * `Image` - String - Required - resource_uid of image
        * `Instance Type` - String - Required - Name of instance type.
        * `User Data` - String - Optional - User Data template for this cloud/image combination.

An Alert definition consists of three fields: a Name, Definition, and Clause (all strings). Clause is a text description of the Alert with this exact format: `If <Metric>.<ValueType> <ComparisonOperator> <Threshold> for <Duration> minutes Then <Action> <ActionValue>`:

* Metric is a collectd metric name such as `cpu-0/cpu-idle`. 
* ValueType is the metric type (`value`, `count`, etc - allowable values differ for each metric so look in the dashboard!). 
* ComparisonOperator is `>`, `>=`, `<`, `<=`, `==`, or `!=`
* Threshold is a numeric value such as `100` or `0.5` or `NaN` for all Metrics except for the RS/* ones. For the RS/* ones its a one of the following server states: `pending`, `booting`, `operational`, `decommission`, `shutting-down`, `terminated`
* Duration is minutes and must be an integer greater than 0.
* Action is either "escalate" in which case the ActionValue is the name of the escalation. Or Action is "grow" or "shrink" in which case ActionValue is the custom tag value to use as the voting tag.

Here is an example ServerTemplate yaml file.

```yaml
Name: My ServerTemplate
Description: This is an example Description
Inputs:
  FIRST_INPUT: "text:overriding value"
  SECOND_INPUT: "env:RS_UUID"
RightScripts:
  Boot:
# Format 3: Name/Revision/Publisher: This specifies a RightScript from the Marketplace
  - Name: RL10 Linux Setup Hostname
    Revision: 6
    Publisher: RightScale
# Format 3: Name/Revision/Publisher: This specifies a RightScript from the Marketplace, latest revision
  - Name: RL10 Enable Monitoring
    Revision: latest
    Publisher: RightScale
# Format 2: Name/Revision: A RightScript in your local account
  - Name: My Local RightScript
    Revision: 3
# Format 1: Locally managed scripts on disk, synced to RightScripts in your local account
  - path/to/script1.sh
  - path/to/script2.sh
  Operational:
  - path/to/script1.sh
  Decommission:
  - path/to/decom_script1.sh
MultiCloudImages:
# Format 1: Name/Revision/Publisher pair: This specifies a MCI from the Marketplace
- Name: Ubuntu_12.04_x64
  Revision: 18
  Publisher: RightScale
# Format 1 again: Name/Revision/Publisher pair: This specifies a latest MCI from the Marketplace
- Name: Ubuntu_14.04_x64
  Revision: latest
  Publisher: RightScale
# Format 2: Name/Revision pair: This specifies a account-specific MCI, such as one cloned from a Marketplace MCI
- Name: Ubuntu_14.04_x64_cloned
  Revision: 20
# Format 3: Href to an account-specific MCI
- Href: /api/multi_cloud_images/403042003
# Format 4: Fully Managed MCI object specifying all clouds/images:
- Name: MyUbuntu_14.04_x64
  Description: My companies custom MCI
  Tags:
  - rs_agent:type=right_link_lite
  - rs_agent:mime_shellscript=https://rightlink.rightscale.com/rll/10/rightlink.boot.sh
  Settings:
  - Cloud: EC2 us-east-1
    Instance Type: m3.medium
    Image: ami-5e91b936
  - Cloud: EC2 eu-west-1
    Instance Type: m3.medium
    Image: ami-b1841cc6
Alerts:
- Name: CPU Scale Down
  Description: Votes to shrink ServerArray by setting tag rs_vote:my_app_name=shrink
  Clause: If cpu-0/cpu-idle.value > '50' for 3 minutes Then shrink my_app_name
- Name: Low memory warning
  Description: Runs escalation named "warning" if free memory drops to < 100MB
  Clause: If memory/memory-free.value < 100000000 for 5 minutes Then escalate warning
```

### ServerTemplate Usage

The following ServerTemplate related commands are supported:

```
right_st st show <name|href|id>
  Show a single ServerTemplate

right_st st upload <path>...
  Upload a ServerTemplate specified by a YAML document
  Flags:
    -x, --prefix: Append a prefix to ServerTemplate and RightScript names when
                  uploading. For creating dev/test versions of ServerTemplates.

right_st st download <name|href|id> [<path>]
  Download a ServerTemplate and all associated RightScripts/Attachments to disk
  Flags:
    -p, --published: When downloading RightScripts, first check if it's published in
                     the MultiCloud marketplace and insert a link to the published
                     script if so.
    -m, --mci-settings: When specifying MultiCloudImages, use Format 4. This fully specifies
                        all cloud/image/instance type settings combinations to completely
                        manage the MultiCloudImage in the YAML.
    -s, --script-path <script-path> Download RightScripts and their attachments
                                    to a subdirectory relative to the download location.

right_st st validate <path>...
  Validate a ServerTemplate YAML document
```

## Contributors

This tool is maintained by [Douglas Thrift (douglaswth)](https://github.com/douglaswth),
[Peter Schroeter (psschroeter)](https://github.com/psschroeter), and [Lopaka Delp (lopaka)](https://github.com/lopaka).

## License

The `right_st` source code is subject to the MIT license, see the
[LICENSE](https://github.com/douglaswth/right_st/blob/master/LICENSE) file.
