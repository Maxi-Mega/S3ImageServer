package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

type Geonames struct {
	Objects []struct {
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
	lastUpdate time.Time
}

func (geonames *Geonames) String() string {
	jsonBytes, err := json.MarshalIndent(geonames, "", "  ") //nolint:musttag
	if err != nil {
		return err.Error()
	}

	return string(jsonBytes)
}

func (geonames *Geonames) format() string {
	var final string

	for _, country := range geonames.Objects {
		final += country.Name + "\n"

		if country.States != nil { //nolint:nestif
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

func (geonames *Geonames) getTopLevel() string {
	if len(geonames.Objects) > 0 { //nolint:nestif
		name := geonames.Objects[0].Name

		states := geonames.Objects[0].States
		if len(states) > 0 {
			name += " / " + states[0].Name

			counties := states[0].Counties
			if len(counties) > 0 {
				name += " / " + counties[0].Name

				cities := counties[0].Cities
				if len(cities) > 0 {
					name += " / " + cities[0].Name
				} // TODO: villages ?
			}
		}

		return name
	}

	return "no geoname found"
}

func (geonames *Geonames) sort() {
	sort.Slice(geonames.Objects, func(i, j int) bool {
		return strings.Compare(strings.ToLower(geonames.Objects[i].Name), strings.ToLower(geonames.Objects[j].Name)) < 0
	})

	for o := range geonames.Objects {
		obj := geonames.Objects[o]
		sort.Slice(obj.States, func(i, j int) bool {
			return strings.Compare(strings.ToLower(obj.States[i].Name), strings.ToLower(obj.States[j].Name)) < 0
		})

		for s := range obj.States {
			state := obj.States[s]
			sort.Slice(state.Counties, func(i, j int) bool {
				return strings.Compare(strings.ToLower(state.Counties[i].Name), strings.ToLower(state.Counties[j].Name)) < 0
			})
		}
	}
}

func parseGeonames(filePath string, objDate time.Time) (Geonames, error) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return Geonames{}, fmt.Errorf("file %q not found", filePath)
		}

		return Geonames{}, err
	}

	var geonames Geonames

	err = json.Unmarshal(fileContent, &geonames.Objects)
	if err != nil {
		return Geonames{}, fmt.Errorf("failed to unmarshal from json the content of the geonames file %q: %w", filePath, err)
	}

	geonames.lastUpdate = objDate
	geonames.sort()

	return geonames, nil
}

func getGeoname(imgName string) string {
	geonamesFilename := imgName[:strings.LastIndex(imgName, "@")+1] + config.GeonamesFilename

	geoname, found := geonamesCache[geonamesFilename]
	if found && len(geoname.Objects) > 0 {
		return geoname.getTopLevel()
	}

	return imgName
}
