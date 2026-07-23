package main

import (
 "encoding/json"
 "net/http"
)

func healthz(w http.ResponseWriter, _ *http.Request) {
 w.Header().Set("Content-Type", "application/json")
 _ = json.NewEncoder(w).Encode(map[string]string{"service": "core", "status": "ok"})
}

func main() {
 mux := http.NewServeMux()
 mux.HandleFunc("GET /healthz", healthz)
 _ = http.ListenAndServe(":8080", mux)
}
