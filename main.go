package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

var ds unsafe.Pointer

func main() {
	label_path := C.CString("bremen-latest.osm.pbf.ce")
	fmt.Printf("Calling init\n")
	ds = C.init(label_path)
	C.free(unsafe.Pointer(label_path))
	fmt.Printf("init finished\n")
	good := C.is_good(ds)
	fmt.Printf("Datastructure good: %t\n", good)

	r := mux.NewRouter()
	r.HandleFunc("/hello", hello)
	r.HandleFunc("/label", getLabels)
	log.Fatal(http.ListenAndServe(":8080", r))
}

func hello(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode("Hello")
}

func getLabels(w http.ResponseWriter, r *http.Request) {
	result := C.get_data(ds, 0.001, 8.0, 9.0, 53.0, 54.0)
	labels := resultToLabels(result)
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
