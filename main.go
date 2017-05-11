package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"unsafe"

	"github.com/gorilla/mux"
)

/*

#cgo LDFLAGS: -L./lib -lrt_datastructure
#include "./lib/rt_datastructre.h"
#include <stdlib.h>
*/
import "C"

type Label struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	T     float64 `json:"t"`
	Osmid int64   `json:"id"`
	Prio  int32   `json:"prio"`

	LblFac float64 `json:"lbl_fac"`
	Label  string  `json:"label"`
}

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
	json.NewEncoder(w).Encode(labels)
}

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

// parseStringToCDouble takes a string and convers it
// first into a float (returning an error if this is not possible)
// and then into a C double (C.double)
func parseStringToCDouble(s string) (C.double, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	d := C.double(f)
	return d, nil
}

// tryParsingFormValue gets the form value fromKey from r and tries
// to convert it to double. If this fails, an error message will be written
// to w and an error will be returned to the caller
func tryParsingFormValue(w http.ResponseWriter, r *http.Request, formKey string) (C.double, error) {

	d, err := parseStringToCDouble((r.FormValue(formKey)))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(formKey + " could not be parsed into a number")
		return 0, err
	}
	return d, nil
}
