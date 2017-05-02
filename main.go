package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

type CLabel struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	T     float64 `json:"t"`
	Osmid int64   `json:"id"`
	Prio  int32   `json:"prio"`

	Lbl_fac float64 `json:"lbl_fac"`
	Label   string  `json:"label"`
}

func main() {
	//	var ds *interface{}

	r := mux.NewRouter()
	r.HandleFunc("/hello", hello)
	log.Fatal(http.ListenAndServe(":8080", r))
}

func hello(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode("Hello")
}
