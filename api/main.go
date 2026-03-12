package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	log.Println("Starting API server on port 8081...")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello World!")
	})

	http.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "This is some data from the API server.")
	})

	log.Println("API server is running on port 8081...")
	http.ListenAndServe(":8081", nil) // nosemgrep: go.lang.security.audit.net.use-tls.use-tls
}