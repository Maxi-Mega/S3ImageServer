package main

import (
	_ "embed"
	"errors"
	"html/template"
)

//go:embed resources/html_body.tmpl
var htmlBodyTemplate string

//go:embed resources/index_ws.tmpl
var indexWSTemplate string

func getIndexWsTemplate() (*template.Template, error) {
	tmpl, err := template.New("index").Parse(indexWSTemplate)
	if err != nil {
		return nil, errors.New("Failed to parse index WS template: " + err.Error())
	}

	tmpl, err = tmpl.Parse(htmlBodyTemplate)
	if err != nil {
		return nil, errors.New("Failed to parse html body template: " + err.Error())
	}

	return tmpl, nil
}
