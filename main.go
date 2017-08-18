// Copyright (C) {2017}  {Florian Barth florianbarth@gmx.de, Matthias Wagner matzew.mail@gmail.com, Marc Schubert marcschubert1@gmx.de}
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
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"os"
	"os/signal"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/mux"
)

/*

#cgo LDFLAGS: -L./lib -lrt_datastructure
#include "./lib/rt_datastructre.h"
#include <stdlib.h>
*/
import "C"

// Label is a go representation of the labels that are returned by the
// label-library
type Label struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	T     float64 `json:"t"`
	Osmid int64   `json:"id"`
	Prio  int32   `json:"prio"`

	LblFac float64 `json:"lbl_fac"`
	Label  string  `json:"label"`
}

// dsMap contains the datastructures which the label-library creates and
// uses. It can only be created through the functions C.get_data and
// C.is_good.
var dsMap map[string]*C.Datastructure

// pRootEndpoint is the path prefix for all label collection
var pRootEndpoint string

// renderdConfigPath contains the path to renderd.conf. It is used for
// the labelCollections endpoint
var renderdConfigPath string

// dsLock contains a ReadWrite Mutex for every configured endpoint.
var dsLock map[string]*sync.RWMutex

func main() {

	var paramLabel string
	var pPort int
	flag.StringVar(&paramLabel, "endpoints", "default.json", "Path to the file with label files and the endpoints where they are supplied.")
	flag.IntVar(&pPort, "port", 8080, "Port where the server is reachable")
	flag.StringVar(&pRootEndpoint, "root", "label", "Endpoint name prefix for all the services")
	flag.StringVar(&renderdConfigPath, "renderd", "/usr/local/etc/renderd.conf", "Path to renderd.conf used to parse urls of tiles")
	flag.Parse()

	// Flag validation
	if pPort <= 0 || pPort > 65535 {
		log.Printf("Port not in allowed range. Cannot start with that configuration. Please use a free port out of [1, 65535].")
		return
	}

	// Read configuration for endpoints from file
	endpointConfigs, err := getEndpointConfig(paramLabel)
	if err != nil {
		log.Printf("Read config failed. No endpoints set. Shutting down.")
		return
	}

	if len(endpointConfigs) == 0 {
		log.Printf("No endpoints configured. Shutting down.")
		return
	}

	// Set up datastructures with the endpoint setups
	dsMap = map[string]*C.Datastructure{}
	for _, conf := range endpointConfigs {
		log.Printf("Init for %s with path %s\n", conf.Name, conf.Path)
		cLabel := C.CString(conf.Path)
		datastructure := C.init(cLabel)
		C.free(unsafe.Pointer(cLabel))
		cdIsGood := C.is_good(datastructure)
		if cdIsGood {
			log.Printf("Init successful. Available endpoint: %s", conf.Name)
			dsMap[conf.Name] = datastructure
		} else {
			log.Printf("Init failed. Data for %s not available.", conf.Name)
		}
	}

	dsLock = make(map[string]*sync.RWMutex, len(endpointConfigs))
	for _, conf := range endpointConfigs {
		dsLock[conf.Name] = &sync.RWMutex{}
	}

	// Start watching label files for new versions and reloading them if a new
	// version shows up
	go watchAndReloadLabelData(endpointConfigs)

	// Handle control + C if needed
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println()
		shutdown()
		os.Exit(1)
	}()

	log.Printf("Socket startup at :%d/%s/... ", pPort, pRootEndpoint)
	mainRouter := mux.NewRouter()
	mainRouter.HandleFunc("/labelCollections", getLabelCollections)
	labelRouter := mainRouter.PathPrefix("/" + pRootEndpoint).Subrouter()
	labelRouter.HandleFunc("/{key}", getLabels)

	// http timeout 15 s
	srv := &http.Server{
		Handler:      mainRouter,
		Addr:         ":" + strconv.Itoa(pPort),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

// getLabels is the handler for the endpoint "/label". It parses the
// arguments "x_min", "x_max", "y_min", "y_max" and "t_min" as
// FormValues from the request and passes them to the Label
// library. The obtained result is then transformed into go data types
// and sent to the client json encoded
func getLabels(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Origin", "*")
	xMin, err := tryParsingFormValue(w, r, "x_min")
	if err != nil {
		return
	}
	xMax, err := tryParsingFormValue(w, r, "x_max")
	if err != nil {
		return
	}
	yMin, err := tryParsingFormValue(w, r, "y_min")
	if err != nil {
		return
	}
	yMax, err := tryParsingFormValue(w, r, "y_max")
	if err != nil {
		return
	}
	tMin, err := tryParsingFormValue(w, r, "t_min")
	if err != nil {
		return
	}

	// check if endpoint is valid
	vars := mux.Vars(r)
	_, isKeySet := vars["key"]
	if !isKeySet {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode("No data available. (endpoint name not set)")
		return
	}
	_, isEndpointSet := dsMap[vars["key"]]
	if !isEndpointSet {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode("No data endpoint available. (endpoint does not exist)")
		return
	}

	// request data (including synchronization on read access to dsMap)
	dsLock[vars["key"]].RLock()
	defer dsLock[vars["key"]].RUnlock()
	result := C.get_data(dsMap[vars["key"]], tMin, xMin, xMax, yMin, yMax)
	labels := resultToLabels(result)
	defer C.free_result(result)
	rawJSON, err := json.Marshal(convertToGeo(labels))
	w.Header().Set("Content-Type", "application/json")
	w.Write(rawJSON)
}

// Function for controlled shutdown
func shutdown() {
	log.Println("Shutting down.")
}

type labelCollectionResult struct {
	Root          string         `json:"pathPrefix"`
	Endpoints     []string       `json:"endpoints"`
	TileEndpoints []tileEndpoint `json:"tileEndpoints"`
}

// getLabelCollections returns the path prefix for accessing labels
// and ean array of all label collections that are currently served.
func getLabelCollections(w http.ResponseWriter, r *http.Request) {
	var endPointCount = len(dsMap)
	var endpoints = make([]string, endPointCount)
	counter := 0
	for key := range dsMap {
		endpoints[counter] = key
		counter++
	}
	tileEndpoints, err := parseEndpoints(renderdConfigPath)
	if err != nil {
		log.Printf("Error during parsing: %s ", err.Error())
	}
	labelCollection := labelCollectionResult{
		pRootEndpoint,
		endpoints,
		tileEndpoints,
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Add("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(labelCollection)
}

// watchAndReloadLabelData() watches all label files provided in "endpointConfigs".
// If at least one of them gets modified or replaced, the label file is reloaded.
// While reloading, all requests for the corresponding endpoint have to wait.
func watchAndReloadLabelData(endpointConfigs []endpoint) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)

	// start listening for events in the directories being watched
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				var eventPath string
				if (event.Op&fsnotify.Write == fsnotify.Write) ||
					(event.Op&fsnotify.Create == fsnotify.Create) {
					if event.Name[0:2] == "./" {
						eventPath = event.Name[2:]
					} else {
						eventPath = event.Name
					}
					for _, conf := range endpointConfigs {
						if eventPath == conf.Path {

							// reload file
							log.Printf("Reload for %s with path %s ...", conf.Name, conf.Path)
							cLabel := C.CString(conf.Path)
							datastructure := C.init(cLabel)
							C.free(unsafe.Pointer(cLabel))
							cdIsGood := C.is_good(datastructure)

							if cdIsGood {
								// adopt reloaded label file
								log.Printf("Reload of file %s successful.", conf.Path)
								func() {
									dsLock[conf.Name].Lock()
									defer dsLock[conf.Name].Unlock()
									C.free_datastructure(dsMap[conf.Name]) //TODO: Is this enough to free the datastructure?
									dsMap[conf.Name] = datastructure

								}()
							} else {
								C.free_datastructure(datastructure)
								log.Printf("Reload of file %s failed. Data for %s not available.", conf.Path, conf.Name)
							}
						}
					}
				}

			case err := <-watcher.Errors:
				log.Println("Reload event error: ", err)
			}
		}
	}()

	// Watch all the directories where the provided label files are located
	for _, conf := range endpointConfigs {

		// case: absolute path
		if string(conf.Path[0]) == "/" {
			err = watcher.Add(conf.Path[0:strings.LastIndex(conf.Path, "/")])

			// case: relative path
		} else {
			if strings.Contains(conf.Path, "/") {
				err = watcher.Add(conf.Path[0:strings.LastIndex(conf.Path, "/")])
			} else {
				err = watcher.Add(".")
			}
		}

		if err != nil {
			log.Fatal(err)
		}
	}

	<-done
}
