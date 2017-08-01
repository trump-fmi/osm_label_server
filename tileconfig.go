package main

import (
	"github.com/Terry-Mao/goconf"
)

type tileEndpoint struct {
	Name        string `json:"name"`
	Uri         string `json:"uri"`
	Description string `json:"description"`
}

// parseEndpoints takes a path to a renderd config file and reads the
// endpoints out of it
func parseEndpoints(file string) ([]tileEndpoint, error) {
	var endpoints = make([]tileEndpoint, 0)
	conf := goconf.New()
	conf.Spliter = "="
	conf.Comment = ";"
	if err := conf.Parse(file); err != nil {
		return nil, err
	}

	for _, sectionName := range conf.Sections() {
		section := conf.Get(sectionName)
		uri, err := section.String("URI")
		if err != nil {
			continue
		}
		description, _ := section.String("DESCRIPTION")
		endpoints = append(endpoints, tileEndpoint{sectionName, uri, description})
	}

	return endpoints, nil
}
