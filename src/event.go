package main

import (
	"encoding/json"
	"fmt"
)

const (
	eventAdd    = "ADD"
	eventUpdate = "UPDATE"
	eventRemove = "REMOVE"

	eventGeonames = "GEONAMES"
	eventFeatures = "FEATURES"

	eventReset = "RESET"
)

type EventObject struct {
	ImgType  string   `json:"img_type"`
	ImgKey   string   `json:"img_key"`
	ImgName  string   `json:"img_name"`
	ImgDate  string   `json:"img_date"`
	Features Features `json:"features"`
}

type EventGeonames struct {
	ImgKey   string `json:"img_key"`
	Geonames string `json:"geonames"`
}

type EventFeatures struct {
	ImgKey   string         `json:"img_key"`
	Class    string         `json:"class"`
	Count    int            `json:"featuresCount"`
	Features map[string]int `json:"features"`
}

type event struct {
	EventType string `json:"event_type"`
	EventObj  any    `json:"event_obj"`
	EventDate string `json:"event_date"`
	source    string
}

func (evt event) JSON() []byte {
	data, err := json.Marshal(evt)
	if err != nil {
		printError(fmt.Errorf("failed to marshal event to json: %w", err), false)
	}

	return data
}

//nolint: forcetypeassert,gci,gofmt,goimports
func (evt event) String() string {
	switch evt.EventType {
	case eventAdd:
		return evt.EventType + ":" + evt.EventObj.(EventObject).ImgKey + "_" + evt.EventDate
	case eventUpdate:
		return evt.EventType + ":" + evt.EventObj.(EventObject).ImgKey + "_" + evt.EventDate
	case eventRemove:
		return evt.EventType + ":" + evt.EventObj.(EventObject).ImgKey
	case eventGeonames:
		return evt.EventType + ":" + evt.EventObj.(EventGeonames).ImgKey
	case eventFeatures:
		return evt.EventType + ":" + evt.EventObj.(EventFeatures).ImgKey
	default:
		printWarn("[event String()] Unknown event type: ", evt.EventType)

		return "Unknown event type:" + evt.EventType
	}
}
