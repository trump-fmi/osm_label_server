// Copyright (C) {2017}  {Florian Barth florianbarth@gmx.de, Matthias Wagner matzew.mail@gmail.com}
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
	"net/http"
	"strconv"
	"unsafe"

	gj "github.com/kpawlik/geojson"
)

/*

#cgo LDFLAGS: -L./lib -lrt_datastructure
#include "./lib/rt_datastructre.h"
#include <stdlib.h>
*/
import "C"

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
		props := map[string]interface{}{"name": l.Label, "t": l.T, "prio": l.Prio, "osm": l.Osmid, "lbl_fac": l.LblFac}
		feat = gj.NewFeature(g, props, l.Osmid)
		fcol.AddFeatures(feat)
	}
	// Add coordinate system definition for openlayers
	fcol.Crs = gj.NewNamedCRS("urn:ogc:def:crs:OGC:1.3:CRS84")
	return fcol
}
