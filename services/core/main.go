package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"

	"github.com/Moussa6503Mohamed/Sidus/services/core/internal/contentsource"
)

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"service": "core", "status": "ok"})
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)

	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			log.Fatalf("open database: %v", err)
		}
		defer db.Close()
		contentsource.Register(mux, contentsource.NewPostgresStore(db))
	} else {
		log.Println("DATABASE_URL not set: content-sources endpoints are disabled")
	}

	_ = http.ListenAndServe(":8080", mux)
}
