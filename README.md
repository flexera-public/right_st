# right_st

right_st is a tool for managing RightScale ServerTemplate and RightScripts. The tool is able to upload and download ServerTemplate and RightScripts using RightScale's API. This tool can easily be hooked into Travis CI or other build systems to manage these design objects if stored in Github. See below for usage examples

## Managing RightScripts

RightScripts consist of a script body, attachments, and metadata describing the inputs, description, and name. Metadata is expected to be embedded as a comment at the top of the RightScript. Metadata format is as follows:
| Field | Format | Description |
| ----- | ------ | ----------- |
| RightScript Name | String | Name of RightScript. Name must be unique for your account. |
| Description | String | Description field for the RightScript |
| Inputs | Hash of String -> Input | The hash is key is the input name. The hash value is an Input definition (defined below) |
| Attachments | Array of Strings | Each string is a path (relative to the .yml) pointing to the attachment file |

Input definition format is as follows:
| Field | Format | Description |
| ----- | ------ | ----------- |
| Input Type | String | "single" or "array" |
| Category | String | Category to group Input under |
| Description | Description field for the Input |
| Default | String | Default value. Value must be in [Inputs 2.0 format](http://reference.rightscale.com/api1.5/resources/ResourceInputs.html) from RightScale API which consists of a type followed by a colon then the value. I.e. "text:Foo", "ignore", "env:ENV_VAR" |
| Required | Boolean(true|false) | Whether or not the Input is required |
| Advanced | Boolean(true|false) | Whether or not the Input is advanced (hidden by default) |


Example RightScript is as follows:
```bash
#! /bin/bash -e

# ---
# RightScript Name: Run Foo Tool
# Description: Runs attached foo executable with input
# Inputs:
#   FOO_PARAM:
#     Input Type: single
#     Category: RightScale
#     Description: A parameter to the foo tool
#     Default: "text:bar"
#     Required: false
#     Advanced: true
# Attachments:
#   - attachments/foo
# ...
#

cp -f $RS_ATTACH_DIR/foo /usr/local/bin/foo
chmod a+x /usr/local/bin/foo
foo $FOO_PARAM
```


## Managing ServerTemplates

ServerTemplates are defined by a YAML format representing the ServerTemplate. The following keys are supported:
| Field | Format | Description |
| ----- | ------ | ----------- |
| Name | String | Name of the ServerTemplate. Name must be unique for your account. |
| Description | String | Description field for the ServerTemplate. |
| RightScripts | Hash | The hash key is the sequence type, one of "Boot", "Operational", or "Decommission". The hash value is a array of strings, where each string is a relative pathname to a RightScript on disk. |
| Inputs | Hash of String -> Input | The hash key is the input name. The hash value is the Input definition (defined above). |
| MultiCloudImages | Array of MultiCloudImages | An array of MultiCloudImage definitions. A MultiCloudImage definition is a hash specifying a MCI. Currently the only MultiCloudImage definition format is by Href. See example below |

Example format is as follows:
```yaml
---
Name: My ServerTemplate
Description: This is an example Description
RightScripts:
  Boot:
    - path/to/script1.sh
    - path/to/script2.sh
  Operational:
    - path/to/script1.sh
  Decommission:
    - path/to/decom_script1.sh
Inputs:
  FIRST_INPUT:
    Input Type: single
    Category: RightScale
    Description: Example Description
    Default: "text:default value"
    Required: false
    Advanced: true
MultiCloudImages:
  - Href: /api/multi_cloud_images/403042003
```

## Contributors

This tool is maintained by [Douglas Thrift (douglaswth)](https://github.com/douglaswth),
[Peter Schroeter (psschroeter)](https://github.com/psschroeter), and [Lopaka Delp (lopaka)](https://github.com/lopaka).

## License

The `right_st` source code is subject to the MIT license, see the
[LICENSE](https://github.com/douglaswth/right_st/blob/master/) file.
