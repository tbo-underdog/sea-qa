package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync/atomic"
)

func main() {
	var cleanupCount int32

	mux := http.NewServeMux()

	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var in struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "u-123",
			"email": in.Email,
			"name":  in.Name,
		})
	})

	mux.HandleFunc("/fail", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	mux.HandleFunc("/cleanup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&cleanupCount, 1)
		w.WriteHeader(http.StatusNoContent)
	})

	addr := ":8081"
	log.Printf("apimock listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
