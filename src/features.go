package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

type Features map[string]uint

func (features Features) toJson() string {
	if features == nil {
		return "{}"
	}
	result, err := json.Marshal(features)
	if err != nil {
		printError(fmt.Errorf("failed to marshal features to json: %v", err), false)
		return "{}"
	}
	return string(result)
}

func parseFeatures(filePath string) (Features, error) {
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
		return Features{}, fmt.Errorf("failed to unmarshal from json the content of the features file %q: %v", filePath, err)
	}

	features := make(Features)
	for _, rawFeature := range rawFeatures.Features {
		detection := strings.Title(rawFeature.Properties[config.FeaturesPropertyName].(string))
		// TODO: inflection ?
		if !strings.HasSuffix(detection, "s") {
			detection += "s"
		}
		if count, found := features[detection]; found {
			features[detection] = count + 1
		} else {
			features[detection] = 1
		}
	}
	return features, nil
}
