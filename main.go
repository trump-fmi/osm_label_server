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
	"fmt"
	"log"
	"net/http"
	"strconv"
	"unsafe"

	"github.com/gorilla/mux"
	"github.com/paulmach/go.geojson"
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

// ds contains the datastructure which the label-library creates and
// uses. It can only be used through the functions C.get_data and
// C.is_good.
var ds *C.Datastructure

func main() {
	label_path := C.CString("bremen-latest.osm.pbf.ce")
	fmt.Printf("Calling init\n")
	ds = C.init(label_path)
	C.free(unsafe.Pointer(label_path))
	fmt.Printf("init finished\n")
	good := C.is_good(ds)
	fmt.Printf("Datastructure good: %t\n", good)

	r := mux.NewRouter()
	r.HandleFunc("/label", getLabels)
	log.Fatal(http.ListenAndServe(":8080", r))
}

// getLabels is the handler for the endpoint "/label". It parses the
// arguments "x_min", "x_max", "y_min", "y_max" and "t_min" as
// FormValues from the request and passes them to the Label
// library. The obtained result is then transformed into go data types
// and sent to the client json encoded
func getLabels(w http.ResponseWriter, r *http.Request) {
	x_min, err := tryParsingFormValue(w, r, "x_min")
	if err != nil {
		return
	}
	x_max, err := tryParsingFormValue(w, r, "x_max")
	if err != nil {
		return
	}
	y_min, err := tryParsingFormValue(w, r, "y_min")
	if err != nil {
		return
	}
	y_max, err := tryParsingFormValue(w, r, "y_max")
	if err != nil {
		return
	}
	t_min, err := tryParsingFormValue(w, r, "t_min")
	if err != nil {
		return
	}
	result := C.get_data(ds, t_min, x_min, x_max, y_min, y_max)
	labels := resultToLabels(result)

	C.free_result(result)
	w.Header().Add("Access-Control-Allow-Origin", "*")
//	json.NewEncoder(w).Encode(labels)
	rawJSON, err := convertToGeo(labels).MarshalJSON()
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

// Convert Labels to geoJson objects
func convertToGeo(labels []Label) *geojson.FeatureCollection {
	var fcol 	*geojson.FeatureCollection = geojson.NewFeatureCollection()
	var g 		*geojson.Geometry
	var feat 	*geojson.Feature

	for _, l := range labels {
		g = geojson.NewPointGeometry([]float64{l.X, l.Y})
		feat = geojson.NewFeature(g)
		feat.Properties["name"] = l.Label
		feat.Properties["t"] = l.T
		feat.Properties["prio"] = l.Prio
		feat.Properties["osm"] = l.Osmid
		fcol.AddFeature(feat)
	}
	return fcol
}
