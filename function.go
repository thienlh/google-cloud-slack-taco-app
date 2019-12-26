// Package p contains an HTTP Cloud Function.
package p

import (
	"bytes"
	"log"
	"net/http"
)

// Handle handle every requests
func Handle(w http.ResponseWriter, r *http.Request) {
	buffer := new(bytes.Buffer)
	_, err := buffer.ReadFrom(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatal("Error reading buffer from body.")
		return
	}
	log.Printf("Header: %v\n", r.Header)
	body := buffer.String()
	log.Printf("Body: %v\n", body)
	succeed := parseEvent(body, w)
	if !succeed {
		log.Printf("Unable to parse event. Error %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	log.Printf("Done")
	return
}
