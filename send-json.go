package main

import "net/http"
import "encoding/json"
import "log"

func sendJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON: %v", err)
	}
}
