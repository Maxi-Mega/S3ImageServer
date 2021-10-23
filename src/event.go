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

	eventReset = "RESET"
)

type EventObject struct {
	ImgType string `json:"img_type"`
	ImgKey  string `json:"img_key"`
	ImgName string `json:"img_name"`
}

type EventGeonames struct {
	ImgKey   string `json:"img_key"`
	Geonames string `json:"geonames"`
}

type event struct {
	EventType string      `json:"event_type"`
	EventObj  interface{} `json:"event_obj"`
	EventDate string      `json:"event_date"`
}

func (evt event) Json() []byte {
	data, err := json.Marshal(evt)
	if err != nil {
		printError(err, false)
	}
	return data
}

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
	default:
		fmt.Println("[event String()] Unknown event type:", evt.EventType)
		return "Unknown event type:" + evt.EventType
	}
}
