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
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"unsafe"

	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
	gj "github.com/kpawlik/geojson"
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

func main() {

	var paramLabel string
	var pRootEndpoint string
	var pPort int
	flag.StringVar(&paramLabel, "ce", "bremen-latest.osm.pbf.ce", "Path to the file with the labels to supply. Should be a 'ce' file.")
	flag.IntVar(&pPort, "port", 8080, "Port where the server is reachable")
	flag.StringVar(&pRootEndpoint, "root", "label", "Endpoint name prefix for all the services")
	flag.Parse()

	// TODO Read from config file
	tmpInput := map[string]string{"city": "bremen-latest.osm.pbf.ce", "bike": "bremen-latest-2.osm.pbf.ce"}

	dsMap = map[string]*C.Datastructure{}
	for k, v := range tmpInput {
		log.Printf("Init for %s with path %s\n", k, v)
		cLabel := C.CString(v)
		cDs := C.init(cLabel)
		C.free(unsafe.Pointer(cLabel))
		cdIsGood := C.is_good(cDs)
		if cdIsGood {
			log.Printf("Init successful. Available endpoint: %s", k)
			dsMap[k] = cDs
		} else {
			log.Printf("Init failed. Data for %s not available.", k)
		}
	}

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
	r := mux.NewRouter().PathPrefix("/" + pRootEndpoint).Subrouter()
	r.HandleFunc("/{key}", getLabels)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(pPort), r))
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
	vars := mux.Vars(r)
	_, isKeySet := vars["key"]
	_, isEndpointSet := dsMap[vars["key"]]
	if !isKeySet || !isEndpointSet {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode("No data available.")
		return
	}
	result := C.get_data(dsMap[vars["key"]], tMin, xMin, xMax, yMin, yMax)
	if result.error != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(C.GoString(result.error))
		C.free_result(result)
		return
	}
	labels := resultToLabels(result)
	C.free_result(result)
	rawJSON, err := json.Marshal(convertToGeo(labels))
	w.Header().Set("Content-Type", "application/json")
	w.Write(rawJSON)
}

// cLabelToLabel takes a pointer to a C_Label given by the
// label-library and turns it into the go-struct Label
func cLabelToLabel(clabel *C.C_Label) Label {
	label := Label{
		float64(clabel.x),
		float64(clabel.y),
		float64(clabel.t),
		int64(clabel.osm_id),
		int32(clabel.prio),
		float64(clabel.lbl_fac),
		C.GoString(clabel.label),
	}
	return label
}

// resultToLabelt iterates over the C-style array stored in C_Result
// and converts it into a slice of Label
func resultToLabels(result C.C_Result) []Label {
	labels := make([]Label, result.size)
	start := uintptr(unsafe.Pointer(result.data))
	arrSize := int(result.size)
	size := unsafe.Sizeof(*result.data)
	for i := 0; i < arrSize; i++ {
		item := (*C.C_Label)(unsafe.Pointer(start + size*uintptr(i)))
		labels[i] = cLabelToLabel(item)
	}
	return labels
}

// parseStringToCDouble takes a string and convers it first into a
// float (returning an error if this is not possible) and then into a
// C double (C.double)
func parseStringToCDouble(s string) (C.double, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	d := C.double(f)
	return d, nil
}

// tryParsingFormValue gets the form value fromKey from r and tries to
// convert it to double. If this fails, an error message will be
// written to w and an error will be returned to the caller
func tryParsingFormValue(w http.ResponseWriter, r *http.Request, formKey string) (C.double, error) {

	d, err := parseStringToCDouble((r.FormValue(formKey)))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(formKey + " could not be parsed into a number")
		return 0, err
	}
	return d, nil
}

// Converts Labels objects to geoJson objects and adds them to the returned feature collection object.
func convertToGeo(labels []Label) *gj.FeatureCollection {
	var fcol = gj.NewFeatureCollection([]*gj.Feature{})
	var g *gj.Point
	var feat *gj.Feature

	for _, l := range labels {
		g = gj.NewPoint(gj.Coordinate{gj.Coord(l.X), gj.Coord(l.Y)})
		// If additional information is needed, add them here
		props := map[string]interface{}{"name": l.Label, "t": l.T, "prio": l.Prio, "osm": l.Osmid}
		feat = gj.NewFeature(g, props, nil)
		fcol.AddFeatures(feat)
	}
	// Add coordinate system definition for openlayers
	fcol.Crs = gj.NewNamedCRS("urn:ogc:def:crs:OGC:1.3:CRS84")
	return fcol
}

// Function for controlled shutdown
func shutdown() {
	log.Println("Shutting down.")
}
