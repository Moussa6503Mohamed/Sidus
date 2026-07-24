package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/lib/pq"

	"github.com/Moussa6503Mohamed/Sidus/services/core/internal/auth"
	"github.com/Moussa6503Mohamed/Sidus/services/core/internal/contentsource"
)

// defaultAuthorizedParty is the only `azp` origin accepted when CLERK_AUTHORIZED_PARTIES is
// not set: the local Sidus web origin. Production must set the env explicitly.
const defaultAuthorizedParty = "http://localhost:3000"

// clerkVerifierFromEnv builds the Clerk session verifier from the environment. It returns nil
// (and logs which variable is missing) when Clerk is not configured, so the caller can
// fail closed rather than expose unauthenticated content-source endpoints. Secret values are
// never logged.
func clerkVerifierFromEnv() auth.Verifier {
	secret := os.Getenv("CLERK_SECRET_KEY")
	if secret == "" {
		log.Println("CLERK_SECRET_KEY not set: content-sources endpoints stay disabled (auth required)")
		return nil
	}

	parties := []string{defaultAuthorizedParty}
	if raw := os.Getenv("CLERK_AUTHORIZED_PARTIES"); raw != "" {
		parties = splitAndTrim(raw)
	}

	return auth.NewClerkVerifier(auth.ClerkConfig{
		SecretKey:         secret,
		Issuer:            os.Getenv("CLERK_JWT_ISSUER"),
		AuthorizedParties: parties,
	})
}

// splitAndTrim splits a comma-separated list into non-empty, trimmed entries.
func splitAndTrim(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"service": "core", "status": "ok"})
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)

	dsn := os.Getenv("DATABASE_URL")
	verifier := clerkVerifierFromEnv()
	switch {
	case dsn == "":
		log.Println("DATABASE_URL not set: content-sources endpoints are disabled")
	case verifier == nil:
		// Fail closed: never mount content-source endpoints without a session verifier.
		log.Println("Clerk not configured: content-sources endpoints stay disabled")
	default:
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			log.Fatalf("open database: %v", err)
		}
		defer db.Close()
		contentsource.Register(mux, contentsource.NewPostgresStore(db), verifier)
	}

	_ = http.ListenAndServe(":8080", mux)
}
