package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Geonames []struct {
	Name   string `json:"name"`
	States []struct {
		Name     string `json:"name"`
		Counties []struct {
			Name   string `json:"name"`
			Cities []struct {
				Name string `json:"name"`
			} `json:"cities"`
			Villages []struct {
				Name string `json:"name"`
			} `json:"villages"`
		} `json:"counties"`
	} `json:"states"`
}

func (geonames Geonames) String() string {
	jsonBytes, err := json.MarshalIndent(geonames, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(jsonBytes)
}

func (geonames Geonames) format() string {
	var final string

	for _, country := range geonames {
		final += country.Name + "\n"
		if country.States != nil {
			for _, state := range country.States {
				final += "  " + state.Name + "\n"
				if state.Counties != nil {
					for _, county := range state.Counties {
						final += "    " + county.Name + "\n"
						if county.Cities != nil {
							for _, city := range county.Cities {
								final += "      " + city.Name + "\n"
							}
						}
						if county.Villages != nil {
							for _, village := range county.Villages {
								final += "        " + village.Name + "\n"
							}
						}
					}
				}
			}
		}
	}

	return final
}

func (geonames Geonames) getTopLevel() string {
	if len(geonames) > 0 {
		name := geonames[0].Name
		states := geonames[0].States
		if states != nil && len(states) > 0 {
			name += " / " + states[0].Name
			counties := states[0].Counties
			if counties != nil && len(counties) > 0 {
				name += " / " + counties[0].Name
				cities := counties[0].Cities
				if cities != nil && len(cities) > 0 {
					name += " / " + cities[0].Name
				} // TODO: villages ?
			}
		}
		return name
	}
	return "no geoname found"
}

func parseGeonames(filePath string) (Geonames, error) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return Geonames{}, fmt.Errorf("file %q not found", filePath)
		}
		return Geonames{}, err
	}

	var geonames Geonames
	err = json.Unmarshal(fileContent, &geonames)
	if err != nil {
		return Geonames{}, fmt.Errorf("failed to unmarshal from json the content of the geonames file %q: %v", filePath, err)
	}

	return geonames, nil
}

func getGeoname(imgName string) string {
	// geonamesFilename := strings.TrimSuffix(imgName, config.PreviewFilename) + config.GeonamesFilename
	geonamesFilename := imgName[:strings.LastIndex(imgName, "@")+1] + config.GeonamesFilename
	geoname, found := geonamesCache[geonamesFilename]
	if found && len(geoname) > 0 {
		return geoname.getTopLevel()
	}
	return imgName
}
