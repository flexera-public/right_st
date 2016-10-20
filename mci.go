package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rightscale/rsc/cm15"
	"github.com/rightscale/rsc/rsapi"
)

type Setting struct {
	Cloud            string `yaml:"Cloud"`
	InstanceType     string `yaml:"Instance Type"`
	Image            string `yaml:"Image"`
	UserData         string `yaml:"User Data,omitempty"`
	cloudHref        string
	instanceTypeHref string
	imageHref        string
}

type MultiCloudImage struct {
	Href        string     `yaml:"Href,omitempty"`
	Name        string     `yaml:"Name,omitempty"`
	Description string     `yaml:"Description,omitempty"`
	Revision    RsRevision `yaml:"Revision,omitempty"`
	Publisher   string     `yaml:"Publisher,omitempty"`
	Tags        []string   `yaml:"Tags,omitempty"`
	// Settings are like MultiCloudImageSettings, defining cloud/resource_uid sets
	Settings []*Setting `yaml:"Settings,omitempty"`
}

type RsRevision int

// Cache lookups for below
var cloudsLookup []*cm15.Cloud
var instanceTypesLookup map[string][]*cm15.InstanceType

// Let people specify MCIs multiple ways:
//   1. Href (Sort of there for completeness and to break ties for 2. may remove at some point)
//   2. Name/Revision pair (similar to above, but at least somewhat portable)
//   3. Name/Revision/Publisher triplet (preferred -- more portable)
//   4. Set of Images + Tags to support autocreation/management of MCIs
func validateMultiCloudImage(mciDef *MultiCloudImage) (errors []error) {
	client, _ := Config.Account.Client15()

	if instanceTypesLookup == nil {
		cl, err := client.CloudLocator("/api/clouds").Index(rsapi.APIParams{})
		if err != nil {
			fatalError("Could not execute API call to get clouds: %s", err.Error())
		}
		cloudsLookup = cl
		instanceTypesLookup = make(map[string][]*cm15.InstanceType)
	}

	if mciDef.Href != "" {
		loc := client.MultiCloudImageLocator(mciDef.Href)
		mci, err := loc.Show()
		if err != nil {
			errors = append(errors, fmt.Errorf("Could not find MCI HREF %s in account", mciDef.Href))
		}
		mciDef.Name = mci.Name
		mciDef.Revision = RsRevision(mci.Revision)
	} else if len(mciDef.Settings) > 0 {
		for i, s := range mciDef.Settings {
			if s.Cloud == "" || s.InstanceType == "" || s.Image == "" {
				errors = append(errors, fmt.Errorf("Invalid setting, Cloud, Instance Type, and Image fields must be set\n"))
				return
			}
			for _, c := range cloudsLookup {
				if getLink(c.Links, "self") == s.Cloud || c.DisplayName == s.Cloud || c.Name == s.Cloud {
					mciDef.Settings[i].cloudHref = getLink(c.Links, "self")
				}
			}
			if mciDef.Settings[i].cloudHref == "" {
				errors = append(errors, fmt.Errorf("Cannot find cloud %s for MCI '%s' Setting #%d",
					s.Cloud, mciDef.Name, i+1))
				return
			}
			if _, ok := instanceTypesLookup[mciDef.Settings[i].cloudHref]; !ok {
				its, err := client.InstanceTypeLocator(mciDef.Settings[i].cloudHref + "/instance_types").Index(rsapi.APIParams{})
				if err != nil {
					errors = append(errors, fmt.Errorf("WARNING: Could not complete API call: %s\n", err.Error()))
				}
				instanceTypesLookup[mciDef.Settings[i].cloudHref] = its
			}
			for _, it := range instanceTypesLookup[mciDef.Settings[i].cloudHref] {
				if getLink(it.Links, "self") == s.InstanceType || it.Name == s.InstanceType || it.ResourceUid == s.InstanceType {
					mciDef.Settings[i].instanceTypeHref = getLink(it.Links, "self")
				}
			}
			if mciDef.Settings[i].instanceTypeHref == "" {
				errors = append(errors, fmt.Errorf("Cannot find instance type %s for MCI '%s' cloud %s",
					s.InstanceType, mciDef.Name, mciDef.Settings[i].Cloud))
			}

			apiParams := rsapi.APIParams{"filter": []string{"resource_uid==" + s.Image}}
			images, err := client.ImageLocator(mciDef.Settings[i].cloudHref + "/images").Index(apiParams)
			if err != nil {
				errors = append(errors, fmt.Errorf("WARNING: Could not complete API call for MCI '%s' cloud %s: : %s\n",
					err.Error(), mciDef.Name, mciDef.Settings[i].Cloud))
			}
			if len(images) < 1 {
				errors = append(errors, fmt.Errorf("Cannot find image with resource_uid %s for MCI '%s' cloud %s",
					s.Image, mciDef.Name, mciDef.Settings[i].Cloud))
			} else {
				mciDef.Settings[i].imageHref = getLink(images[0].Links, "self")
			}

		}
	} else if mciDef.Publisher != "" {
		pub, err := findPublication("MultiCloudImage", mciDef.Name, int(mciDef.Revision),
			map[string]string{`Publisher`: mciDef.Publisher})
		if err != nil {
			errors = append(errors, fmt.Errorf("Error finding publication for MultiCloudImage: %s\n", err.Error()))
		}
		if pub == nil {
			errors = append(errors, fmt.Errorf("Could not find a publication in the MultiCloud Marketplace for MultiCloudImage '%s' Revision %s Publisher '%s'",
				mciDef.Name, formatRev(int(mciDef.Revision)), mciDef.Publisher))
		}
	} else if mciDef.Name != "" {
		href, err := paramToHref("multi_cloud_images", mciDef.Name, int(mciDef.Revision))
		if err != nil {
			errors = append(errors, fmt.Errorf("Could not find MCI named '%s' with revision %s in account",
				mciDef.Name, formatRev(int(mciDef.Revision))))
		}
		mciDef.Href = href
	} else {
		errors = append(errors, fmt.Errorf("MultiCloudImage item must be a hash with Settings, Name/Revision, or Name/Revision/Publisher keys set to a valid values."))
	}
	return
}

func downloadMultiCloudImages(st *cm15.ServerTemplate, downloadMciSettings bool) ([]*MultiCloudImage, error) {
	client, _ := Config.Account.Client15()

	defaultMciHref := getLink(st.Links, "default_multi_cloud_image")

	mciLocator := client.MultiCloudImageLocator(getLink(st.Links, "multi_cloud_images"))

	apiMcis, err := mciLocator.Index(rsapi.APIParams{})
	if err != nil {
		return nil, fmt.Errorf("Could not find MCIs with href %s: %s", mciLocator.Href, err.Error())
	}
	mciImages := make([]*MultiCloudImage, 0)
	for _, mci := range apiMcis {
		if downloadMciSettings {
			tags, err := getTagsByHref(getLink(mci.Links, "self"))
			if err != nil {
				return nil, fmt.Errorf("Could not get tags for MultiCloudImage '%s': %s\n", getLink(mci.Links, "self"), err.Error())
			}

			settingsLoc := client.MultiCloudImageSettingLocator(getLink(mci.Links, "settings"))
			settings, err := settingsLoc.Index(rsapi.APIParams{})
			if err != nil {
				return nil, fmt.Errorf("Could not get MultiCloudImage settings %s: %s\n", getLink(mci.Links, "settings"), err.Error())
			}
			mciSettings := make([]*Setting, 0)
			for _, s := range settings {
				cloud, err := client.CloudLocator(getLink(s.Links, "cloud")).Show(rsapi.APIParams{})
				if err != nil {
					if strings.Contains(err.Error(), "ResourceNotFound") {
						fmt.Printf("WARNING: For MCI '%s', skipping setting for cloud %s: cloud isn't registered in this account.\n",
							mci.Name, getLink(s.Links, "cloud"))
						continue
					} else {
						return nil, fmt.Errorf("Could not complete API call for MCI '%s' cloud %s: %s\n",
							mci.Name, getLink(s.Links, "cloud"), err.Error())
					}
				}
				if getLink(s.Links, "instance_type") == "" {
					fmt.Printf("WARNING: For MCI '%s', skipping setting for cloud %s: fingerprinted MCIs not supported by this tool.\n",
						mci.Name, cloud.Name)
					continue
				}
				instanceType, err := client.InstanceTypeLocator(getLink(s.Links, "instance_type")).Show(rsapi.APIParams{})
				if err != nil {
					return nil, fmt.Errorf("Could not complete API call for MCI '%s' cloud %s: %s\n", mci.Name, cloud.Name, err.Error())
				}
				image, err := client.ImageLocator(getLink(s.Links, "image")).Show(rsapi.APIParams{})
				if err != nil {
					fmt.Printf("WARNING: Could not complete API call for MCI '%s' cloud %s: %s\n", mci.Name, cloud.Name, err.Error())
					continue
				}

				mciSetting := Setting{Cloud: cloud.Name, InstanceType: instanceType.ResourceUid, Image: image.ResourceUid}
				mciSettings = append(mciSettings, &mciSetting)
			}
			if len(mciSettings) > 0 {
				// Default MCI is the first in the list
				mciImage := MultiCloudImage{
					Name:        mci.Name,
					Tags:        tags,
					Description: removeCarriageReturns(mci.Description),
					Settings:    mciSettings,
				}
				if getLink(mci.Links, "self") == defaultMciHref {
					mciImages = append([]*MultiCloudImage{&mciImage}, mciImages...)
				} else {
					mciImages = append(mciImages, &mciImage)
				}
			} else {
				fmt.Printf("WARNING: skipping MCI '%s', contains no usable settings\n", mci.Name)
			}
		} else {
			// We repull the MCI here to get the description field, which we need to break ties between
			// similarly named publications!
			mciLoc := client.MultiCloudImageLocator(getLink(mci.Links, "self"))
			mci, err := mciLoc.Show()
			if err != nil {
				return nil, fmt.Errorf("Could not get MultiCloudImage %s: %s\n", getLink(mci.Links, "self"), err.Error())
			}
			pub, err := findPublication("MultiCloudImage", mci.Name, mci.Revision, map[string]string{`Description`: mci.Description})
			if err != nil {
				return nil, fmt.Errorf("Error finding publication: %s\n", err.Error())
			}
			mciImage := MultiCloudImage{Name: mci.Name, Revision: RsRevision(mci.Revision)}
			if pub != nil {
				mciImage.Publisher = pub.Publisher
			}
			// Default MCI is the first in the list
			if getLink(mci.Links, "self") == defaultMciHref {
				mciImages = append([]*MultiCloudImage{&mciImage}, mciImages...)
			} else {
				mciImages = append(mciImages, &mciImage)
			}
		}

	}

	return mciImages, nil
}

func uploadMultiCloudImages(stDef *ServerTemplate, prefix string) error {
	client, _ := Config.Account.Client15()

	stMciLocator := client.ServerTemplateMultiCloudImageLocator("/api/server_template_multi_cloud_images")

	existingMcis, err := stMciLocator.Index(rsapi.APIParams{"filter": []string{"server_template_href==" + stDef.href}})
	if err != nil {
		return fmt.Errorf("Could not find MCIs with href %s: %s", stMciLocator.Href, err.Error())
	}

	// Algorithm for linking Publications:
	//   1. For MultiCloudImages with a publication, find the publications first. Get the name/description/publisher
	//   2. If we don't find it, throw an error
	//   3. Get the imported MultiCloudImages. If it doesn't exist, import it then get the href
	//   4. Insert HREF into r struct for later use.
	for _, mciDef := range stDef.MultiCloudImages {
		if mciDef.Publisher != "" {
			pub, err := findPublication("MultiCloudImage", mciDef.Name, int(mciDef.Revision),
				map[string]string{`Publisher`: mciDef.Publisher})
			if err != nil {
				return fmt.Errorf("Could not lookup publication %s", err.Error())
			}
			if pub == nil {
				return fmt.Errorf("Could not find a publication in the MultiCloud Marketplace for MultiCloudImage '%s' Revision %s Publisher '%s'",
					mciDef.Name, formatRev(int(mciDef.Revision)), mciDef.Publisher)
			}

			mciLocator := client.MultiCloudImageLocator("/api/multi_cloud_images")
			filters := []string{
				"name==" + mciDef.Name,
			}

			mciUnfiltered, err := mciLocator.Index(rsapi.APIParams{"filter": filters})
			if err != nil {
				return fmt.Errorf("Error looking up MCI: %s", err.Error())
			}
			for _, mci := range mciUnfiltered {
				// Recheck the name here, filter does a partial match and we need an exact one.
				// Matching the descriptions helps to disambiguate if we have multiple publications
				// with that same name/revision pair.
				if mci.Name == mciDef.Name && mci.Revision == pub.Revision && mci.Description == pub.Description {
					mciDef.Href = getLink(mci.Links, "self")
				}
			}

			if mciDef.Href == "" {
				loc := pub.Locator(client)

				err = loc.Import()

				if err != nil {
					return fmt.Errorf("Failed to import publication %s for MultiCloudImage '%s' Revision %s Publisher %s\n",
						getLink(pub.Links, "self"), mciDef.Name, formatRev(int(mciDef.Revision)), mciDef.Publisher)
				}

				mciUnfiltered, err := mciLocator.Index(rsapi.APIParams{"filter": filters})
				if err != nil {
					return fmt.Errorf("Error looking up MCI: %s", err.Error())
				}
				for _, mci := range mciUnfiltered {
					if mci.Name == mciDef.Name && mci.Revision == pub.Revision && mci.Description == pub.Description {
						mciDef.Href = getLink(mci.Links, "self")
					}
				}
				if mciDef.Href == "" {
					return fmt.Errorf("Could not refind MultiCloudImage '%s' Revision %s after import!", mciDef.Name, formatRev(pub.Revision))
				}
			}
		}
	}

	// For MCIs managed by us, we create them as needed. All Hrefs to cloud/instance type objects should be resolved
	// during the validation step, so we should be good to go
	for _, mciDef := range stDef.MultiCloudImages {
		if len(mciDef.Settings) > 0 {
			mciName := mciDef.Name
			if prefix != "" {
				mciName = fmt.Sprintf("%s_%s", prefix, mciName)
			}

			href, err := paramToHref("multi_cloud_images", mciName, 0)
			if err != nil && !strings.Contains(err.Error(), "Found no multi_cloud_images matching") {
				return fmt.Errorf("API call to find MultiCloudImage '%s' failed: %s", mciName, err.Error())
			}
			if href == "" {
				createParams := cm15.MultiCloudImageParam{Description: mciDef.Description, Name: mciName}
				loc, err := client.MultiCloudImageLocator("/api/multi_cloud_images").Create(&createParams)
				if err != nil {
					return fmt.Errorf("API call to create MultiCloudImage '%s' failed: %s", mciName, err.Error())
				}
				href = string(loc.Href)
				fmt.Printf("  Created MultiCloudImage with name '%s': %s\n", mciName, href)
			} else {
				mci, err := client.MultiCloudImageLocator(href).Show()
				if err != nil {
					return fmt.Errorf("API call failed: %s", err.Error())
				}
				fmt.Printf("  Updating MultiCloudImage '%s'\n", mciName)
				if mci.Description != mciDef.Description {
					err := mci.Locator(client).Update(&cm15.MultiCloudImageParam{Description: mciDef.Description})
					if err != nil {
						return fmt.Errorf("Failed to update MultiCloudImage '%s' description: %s", mciName, err.Error())
					}
				}
			}
			mciDef.Href = href

			err = setTagsByHref(mciDef.Href, mciDef.Tags)
			if err != nil {
				return fmt.Errorf("Failed to add tags to MultiCloudImage '%s': %s", mciDef.Href, err.Error())
			}
			// get existing settings
			settingsLoc := client.MultiCloudImageSettingLocator(mciDef.Href + "/settings")
			settings, err := settingsLoc.Index(rsapi.APIParams{})
			if err != nil {
				fatalError("Could not get MultiCloudImage settings %s: %s\n", mciDef.Href, err.Error())
			}
			seenSettings := make(map[string]bool)

			for _, s := range mciDef.Settings {
				// for each desired setting, if existing setting with same cloud exists, update it. else add it.
				updated := false
				seenSettings[s.cloudHref] = true
				for _, s2 := range settings {
					if s.cloudHref == getLink(s2.Links, "cloud") {
						updateParams := cm15.MultiCloudImageSettingParam{
							CloudHref:        s.cloudHref,
							ImageHref:        s.imageHref,
							InstanceTypeHref: s.instanceTypeHref,
							UserData:         s.UserData,
							// unsupported: KernelImageHref, RamdiskImageHref
						}

						err := s2.Locator(client).Update(&updateParams)
						if err != nil {
							fatalError("Could not update MultiCloudImage setting %s: %s\n", getLink(s2.Links, "self"), err.Error())
						}
						updated = true
					}
				}
				if !updated {
					createParams := cm15.MultiCloudImageSettingParam{
						CloudHref:        s.cloudHref,
						ImageHref:        s.imageHref,
						InstanceTypeHref: s.instanceTypeHref,
						UserData:         s.UserData,
						// unsupported: KernelImageHref, RamdiskImageHref
					}
					_, err := settingsLoc.Create(&createParams)
					if err != nil {
						fatalError("Could not create MultiCloudImage setting %s: %s\n", mciDef.Href, err.Error())
					}
				}
			}
			// for existing settings not in desired settings, remove them
			for _, s := range settings {
				if !seenSettings[getLink(s.Links, "cloud")] {
					err := s.Locator(client).Destroy()
					if err != nil {
						fatalError("  Could not Remove MCI Setting for MCI '%s' with cloud %s: %s",
							mciName, getLink(s.Links, "cloud"), err.Error())
					}
				}
			}

		}
	}

	// Delete MCIs that exist on the existing ST but not in this definition. We perform all the deletions first so
	// we don't have to worry about reading the same MCI with a different revision and throwing an error later
	// firstValidMci is an MCI we know for sure we're not going to default. We can't delete an MCI marked default so
	// make the firstValidMci the default in that case then proceed with the delete.
	var firstValidMci *cm15.ServerTemplateMultiCloudImageLocator
	for _, mci := range existingMcis {
		mciHref := getLink(mci.Links, "multi_cloud_image")
		for _, mciDef := range stDef.MultiCloudImages {
			if mciDef.Href == mciHref {
				firstValidMci = mci.Locator(client)
				break
			}
		}
		if firstValidMci != nil {
			break
		}
	}
	// Dummy MCI. If we can't find ANY valid Mcis, that means we're going to delete all the existing attached MCIs
	// before adding any new ones. This presents a problem in that the API will return an error if you try and delete
	// the final MCI. We work around this by adding a dummy MCI we later delete
	if firstValidMci == nil {
		mciLocator := client.MultiCloudImageLocator("/api/multi_cloud_images")

		dummyMcis, err := mciLocator.Index(rsapi.APIParams{})
		if err != nil {
			fatalError("Failed to find dummy MCIs: %s", mciLocator.Href, err.Error())
		}
		params := cm15.ServerTemplateMultiCloudImageParam{
			MultiCloudImageHref: getLink(dummyMcis[0].Links, "self"),
			ServerTemplateHref:  stDef.href,
		}
		loc, err := stMciLocator.Create(&params)
		if err != nil {
			fatalError("  Failed to associate Dummy MCI '%s' with ServerTemplate '%s': %s", getLink(dummyMcis[0].Links, "self"), stDef.href, err.Error())
		}
		firstValidMci = loc
		defer loc.Destroy()
	}
	for _, mci := range existingMcis {
		mciHref := getLink(mci.Links, "multi_cloud_image")
		foundMci := false // found on ST definition
		for _, mciDef := range stDef.MultiCloudImages {
			if mciDef.Href == mciHref {
				foundMci = true
			}
		}
		if !foundMci {
			fmt.Printf("  Removing MCI %s\n", mciHref)
			if mci.IsDefault {
				firstValidMci.MakeDefault()
			}
			err := mci.Locator(client).Destroy()
			if err != nil {
				fatalError("  Could not Remove MCI %s", mciHref)
			}
		}
	}

	// Add all MCIs.
	for i, mciDef := range stDef.MultiCloudImages {
		foundMci := false // found on ST
		for _, mci := range existingMcis {
			mciHref := getLink(mci.Links, "multi_cloud_image")
			if mciDef.Href == mciHref {
				foundMci = true
			}
		}
		if !foundMci {
			params := cm15.ServerTemplateMultiCloudImageParam{
				MultiCloudImageHref: mciDef.Href,
				ServerTemplateHref:  stDef.href,
			}
			mciName := mciDef.Name
			if prefix != "" && len(mciDef.Settings) > 0 {
				mciName = fmt.Sprintf("%s_%s", prefix, mciName)
			}
			fmt.Printf("  Adding MCI '%s' revision %s (%s)\n", mciName, formatRev(int(mciDef.Revision)), mciDef.Href)
			loc, err := stMciLocator.Create(&params)
			if err != nil {
				fatalError("  Failed to associate MCI '%s' with ServerTemplate '%s': %s", mciDef.Href, stDef.href, err.Error())
			}
			if i == 0 {
				_ = loc.MakeDefault()
			}
		}
	}
	return nil
}

func deleteMultiCloudImage(mciName string) error {
	client, _ := Config.Account.Client15()

	hrefs, err := paramToHrefs("multi_cloud_images", mciName, 0)
	if err != nil {
		return err
	}
	if len(hrefs) == 0 {
		fmt.Printf("MultiCloudImage '%s' does not exist, no need to delete\n", mciName)
	}
	for _, href := range hrefs {
		loc := client.MultiCloudImageLocator(href)
		fmt.Printf("Deleting MultiCloudImage '%s' HREF %s\n", mciName, href)
		err := loc.Destroy()
		if err != nil {
			return err
		}
	}
	return nil
}

func (rev RsRevision) MarshalYAML() (interface{}, error) {
	if rev == -1 {
		return "latest", nil
	} else if rev == 0 {
		return "head", nil
	} else {
		revInt := int(rev)
		return revInt, nil
	}
}

func (rev *RsRevision) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var strType string
	var intType int
	errorMsg := "Revision must be 'head', 'latest', or an integer"
	err := unmarshal(&strType)
	if err == nil {
		if strType == "latest" {
			*rev = -1
		} else if strType == "head" {
			*rev = 0
		} else {
			revInt, err := strconv.Atoi(strType)
			if err != nil {
				return fmt.Errorf(errorMsg)
			}
			*rev = RsRevision(revInt)
		}
	} else {
		err = unmarshal(&intType)
		if err != nil {
			return fmt.Errorf(errorMsg)
		}
		*rev = RsRevision(intType)
	}

	return nil
}
