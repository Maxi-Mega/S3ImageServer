package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Point struct {
	Coordinates struct {
		Lon float64 `json:"lon"`
		Lat float64 `json:"lat"`
	} `json:"coordinates"`
}

type Localization struct {
	Corner struct {
		UpperLeft  Point `json:"upper-left"`
		UpperRight Point `json:"upper-right"`
		LowerLeft  Point `json:"lower-left"`
		LowerRight Point `json:"lower-right"`
	} `json:"corner"`
	lastUpdate time.Time
}

func parseLocalization(filePath string, objDate time.Time) (Localization, error) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return Localization{}, fmt.Errorf("file %q not found", filePath)
		}
		return Localization{}, err
	}

	var localization Localization
	err = json.Unmarshal(fileContent, &localization)
	if err != nil {
		return Localization{}, fmt.Errorf("failed to unmarshal from json the content of the localization file %q: %v", filePath, err)
	}
	localization.lastUpdate = objDate

	return localization, nil
}
