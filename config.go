// Copyright (C) {2017}  {Florian Barth florianbarth@gmx.de}
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// The osm_label_server delivers an REST endpoint which clients that
// display openstreetmap data can use to obtain labels which can
// be displayed on top of the tiles from the tile-server. This enables
// clients to rotate the map without rotating the labels. It should be
// used as follows
package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

// Endpoint configuration for providing labels
type endpoint struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// getEndpointConfig reads a config file and returns the found endpoint configurations.
// Will return an error if the file reading or the unmarshalling failed
func getEndpointConfig(filePath string) ([]endpoint, error) {

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("Error on reading %s", filePath)
		return nil, err
	}

	var endpoints []endpoint
	marshalErr := json.Unmarshal(data, &endpoints)
	if marshalErr != nil {
		log.Printf("Error on unmarshalling the configuration: %s", marshalErr)
		return nil, marshalErr
	}

	return endpoints, nil
}
