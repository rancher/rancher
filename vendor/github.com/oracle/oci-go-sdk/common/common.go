// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package common

import (
	"fmt"
	"regexp"
	"strings"
)

//Region type for regions
type Region string

const (
	//RegionSEA region SEA
	RegionSEA Region = "sea"
	//RegionCAToronto1 region for toronto
	RegionCAToronto1 Region = "ca-toronto-1"
	//RegionCAMontreal1 region for Montreal
	RegionCAMontreal1 Region = "ca-montreal-1"
	//RegionPHX region PHX
	RegionPHX Region = "us-phoenix-1"
	//RegionIAD region IAD
	RegionIAD Region = "us-ashburn-1"
	//RegionFRA region FRA
	RegionFRA Region = "eu-frankfurt-1"
	//RegionLHR region LHR
	RegionLHR Region = "uk-london-1"
	//RegionAPTokyo1 region for tokyo
	RegionAPTokyo1 Region = "ap-tokyo-1"
	//RegionAPOsaka1 region for Osaka
	RegionAPOsaka1 Region = "ap-osaka-1"
	//RegionAPSeoul1 region for seoul
	RegionAPSeoul1 Region = "ap-seoul-1"
	//RegionAPMumbai1 region for mumbai
	RegionAPMumbai1 Region = "ap-mumbai-1"
	//RegionAPMelbourne1 region for Melbourne
	RegionAPMelbourne1 Region = "ap-melbourne-1"
	//RegionAPSydney1 region for Sydney
	RegionAPSydney1 Region = "ap-sydney-1"
	//RegionMEJeddah1 region for Jeddah
	RegionMEJeddah1 Region = "me-jeddah-1"
	//RegionEUZurich1 region for Zurich
	RegionEUZurich1 Region = "eu-zurich-1"
	//RegionEUAmsterdam1 region for Amsterdam
	RegionEUAmsterdam1 Region = "eu-amsterdam-1"
	//RegionSASaopaulo1 region for Sao Paulo
	RegionSASaopaulo1 Region = "sa-saopaulo-1"
	//RegionUSLangley1 region for langley
	RegionUSLangley1 Region = "us-langley-1"
	//RegionUSLuke1 region for luke
	RegionUSLuke1 Region = "us-luke-1"

	//RegionUSGovAshburn1 region for langley
	RegionUSGovAshburn1 Region = "us-gov-ashburn-1"
	//RegionUSGovChicago1 region for luke
	RegionUSGovChicago1 Region = "us-gov-chicago-1"
	//RegionUSGovPhoenix1 region for luke
	RegionUSGovPhoenix1 Region = "us-gov-phoenix-1"
	//RegionUKGovLondon1 gov region London
	RegionUKGovLondon1 Region = "uk-gov-london-1"
)

var realm = map[string]string{
	"oc1": "oraclecloud.com",
	"oc2": "oraclegovcloud.com",
	"oc3": "oraclegovcloud.com",
	"oc4": "oraclegovcloud.uk",
}

var regionRealm = map[Region]string{
	RegionPHX:          "oc1",
	RegionIAD:          "oc1",
	RegionFRA:          "oc1",
	RegionLHR:          "oc1",
	RegionCAToronto1:   "oc1",
	RegionCAMontreal1:  "oc1",
	RegionAPTokyo1:     "oc1",
	RegionAPOsaka1:     "oc1",
	RegionAPSeoul1:     "oc1",
	RegionAPSydney1:    "oc1",
	RegionAPMumbai1:    "oc1",
	RegionAPMelbourne1: "oc1",
	RegionMEJeddah1:    "oc1",
	RegionEUZurich1:    "oc1",
	RegionEUAmsterdam1: "oc1",
	RegionSASaopaulo1:  "oc1",

	RegionUSLangley1:    "oc2",
	RegionUSLuke1:       "oc2",
	RegionUSGovAshburn1: "oc3",
	RegionUSGovChicago1: "oc3",
	RegionUSGovPhoenix1: "oc3",
	RegionUKGovLondon1:  "oc4",
}

// Endpoint returns a endpoint for a service
func (region Region) Endpoint(service string) string {
	return fmt.Sprintf("%s.%s.%s", service, region, region.secondLevelDomain())
}

// EndpointForTemplate returns a endpoint for a service based on template
func (region Region) EndpointForTemplate(service string, serviceEndpointTemplate string) string {
	if serviceEndpointTemplate == "" {
		return region.Endpoint(service)
	}

	// replace service prefix
	endpoint := strings.Replace(serviceEndpointTemplate, "{serviceEndpointPrefix}", service, 1)

	// replace region
	endpoint = strings.Replace(endpoint, "{region}", string(region), 1)

	// replace second level domain
	endpoint = strings.Replace(endpoint, "{secondLevelDomain}", region.secondLevelDomain(), 1)

	return endpoint
}

func (region Region) secondLevelDomain() string {
	if realmID, ok := regionRealm[region]; ok {
		if secondLevelDomain, ok := realm[realmID]; ok {
			return secondLevelDomain
		}
	}

	Debugf("cannot find realm for region : %s, return default realm value.", region)
	return realm["oc1"]
}

//StringToRegion convert a string to Region type
func StringToRegion(stringRegion string) (r Region) {
	switch strings.ToLower(stringRegion) {
	case "sea":
		r = RegionSEA
	case "yyz", "ca-toronto-1":
		r = RegionCAToronto1
	case "yul", "ca-montreal-1":
		r = RegionCAMontreal1
	case "phx", "us-phoenix-1":
		r = RegionPHX
	case "iad", "us-ashburn-1":
		r = RegionIAD
	case "fra", "eu-frankfurt-1":
		r = RegionFRA
	case "lhr", "uk-london-1":
		r = RegionLHR
	case "nrt", "ap-tokyo-1":
		r = RegionAPTokyo1
	case "kix", "ap-osaka-1":
		r = RegionAPOsaka1
	case "icn", "ap-seoul-1":
		r = RegionAPSeoul1
	case "bom", "ap-mumbai-1":
		r = RegionAPMumbai1
	case "mel", "ap-melbourne-1":
		r = RegionAPMelbourne1
	case "syd", "ap-sydney-1":
		r = RegionAPSydney1
	case "jed", "me-jeddah-1":
		r = RegionMEJeddah1
	case "zrh", "eu-zurich-1":
		r = RegionEUZurich1
	case "ams", "eu-amsterdam-1":
		r = RegionEUAmsterdam1
	case "gru", "sa-saopaulo-1":
		r = RegionSASaopaulo1
	case "us-langley-1":
		r = RegionUSLangley1
	case "us-luke-1":
		r = RegionUSLuke1
	case "us-gov-ashburn-1":
		r = RegionUSGovAshburn1
	case "us-gov-chicago-1":
		r = RegionUSGovChicago1
	case "us-gov-phoenix-1":
		r = RegionUSGovPhoenix1
	case "ltn", "uk-gov-london-1":
		r = RegionUKGovLondon1
	default:
		r = Region(stringRegion)
		Debugf("region named: %s, is not recognized", stringRegion)
	}
	return
}

// canStringBeRegion test if the string can be a region, if it can, returns the string as is, otherwise it
// returns an error
var blankRegex = regexp.MustCompile("\\s")

func canStringBeRegion(stringRegion string) (region string, err error) {
	if blankRegex.MatchString(stringRegion) || stringRegion == "" {
		return "", fmt.Errorf("region can not be empty or have spaces")
	}
	return stringRegion, nil
}
