package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIncompleteGetLabelRequest(t *testing.T) {

	r, _ := http.NewRequest("GET", "http://labels.com/label/city", nil)
	w := httptest.NewRecorder()

	getLabels(w, r)

	if status := w.Code; status != 400 {
		t.Errorf("Got status %d wanted 400", status)
	}

}
