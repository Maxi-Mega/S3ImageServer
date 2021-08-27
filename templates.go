package main

import (
	_ "embed"
	"errors"
	"html/template"
)

//go:embed resources/html_body.tmpl
var htmlBodyTemplate string

//go:embed resources/index.tmpl
var indexTemplate string

//go:embed resources/index_ws.tmpl
var indexWSTemplate string

func getIndexTemplate() (*template.Template, error) {
	tmpl, err := template.New("index").Parse(indexTemplate)
	if err != nil {
		return nil, errors.New("Failed to parse index template: "+err.Error())
	}
	tmpl, err = tmpl.Parse(htmlBodyTemplate)
	if err != nil {
		return nil, errors.New("Failed to parse html body template: "+err.Error())
	}
	return tmpl, nil
}

func getIndexWsTemplate() (*template.Template, error) {
	tmpl, err := template.New("index").Parse(indexWSTemplate)
	if err != nil {
		return nil, errors.New("Failed to parse index WS template: "+err.Error())
	}
	tmpl, err = tmpl.Parse(htmlBodyTemplate)
	if err != nil {
		return nil, errors.New("Failed to parse html body template: "+err.Error())
	}
	return tmpl, nil
}
