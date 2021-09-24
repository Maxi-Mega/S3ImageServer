package main

import (
	"encoding/json"
	"fmt"
)

const (
	eventAdd    = "ADD"
	eventUpdate = "UPDATE"
	eventRemove = "REMOVE"
)

type EventObject struct {
	ImgType string `json:"img_type"`
	ImgKey  string `json:"img_key"`
	ImgName string `json:"img_name"`
}

type event struct {
	EventType string      `json:"event_type"`
	EventObj  EventObject `json:"event_obj"`
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
		return evt.EventType + ":" + evt.EventObj.ImgKey + "_" + evt.EventDate
	case eventUpdate:
		return evt.EventType + ":" + evt.EventObj.ImgKey + "_" + evt.EventDate
	case eventRemove:
		return evt.EventType + ":" + evt.EventObj.ImgKey
	default:
		fmt.Println("Unknown event type:", evt.EventType)
		return "ERROR"
	}
}
