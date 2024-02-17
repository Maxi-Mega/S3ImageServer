package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type RawFeaturesFile struct { // TODO: remove useless fields
	Type     string `json:"type"`
	Features []struct {
		Type string `json:"type"`
		/*Properties struct {
			Detection string `json:"detection"`
		} `json:"properties"`*/
		Properties map[string]interface{} `json:"properties"`
	} `json:"features"`
}

type Features struct {
	Class      string         `json:"class"`
	Count      int            `json:"featuresCount"`
	Objects    map[string]int `json:"objects"`
	lastUpdate time.Time
}

func parseFeatures(filePath string, objDate time.Time) (Features, error) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return Features{}, fmt.Errorf("file %q not found", filePath)
		}

		return Features{}, err
	}

	var rawFeatures RawFeaturesFile

	err = json.Unmarshal(fileContent, &rawFeatures)
	if err != nil {
		return Features{}, fmt.Errorf("failed to unmarshal from json the content of the features file %q: %w", filePath, err)
	}

	features := Features{
		Objects:    make(map[string]int),
		lastUpdate: objDate,
	}

	for i, rawFeature := range rawFeatures.Features {
		category, ok := parseStrProp(config.FeaturesCategoryName, rawFeature.Properties, i, filePath)
		if !ok {
			continue
		}

		class, ok := parseStrProp(config.FeaturesClassName, rawFeature.Properties, i, filePath)
		if !ok {
			continue
		}

		features.Class = class
		features.Count++
		features.Objects[category]++
	}

	return features, nil
}

func parseStrProp(key string, props map[string]any, idx int, filepath string) (string, bool) {
	propName, ok := props[key]
	if !ok {
		logger.Warn().Str("filepath", filepath).Msg(fmt.Sprintf("Feature n°%d has no %s", idx+1, key))
		return "", false
	}

	rawProp, ok := propName.(string)
	if !ok {
		logger.Warn().Str("filepath", filepath).Interface("name", propName).
			Msg(fmt.Sprintf("Feature n°%d %s is not a string", idx+1, key))
		return "", false
	}

	value := cases.Title(language.English).String(rawProp)
	value = strings.ReplaceAll(value, "_", " ")

	return value, true
}
