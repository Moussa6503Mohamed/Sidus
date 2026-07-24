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
// absent: the local Sidus web origin. Production must set the env explicitly to non-local
// origin(s).
const defaultAuthorizedParty = "http://localhost:3000"

// lookupEnv matches os.LookupEnv: it reports the value and whether the variable is present.
// Injected in tests so configuration behavior is verified without touching the process env.
type lookupEnv func(key string) (string, bool)

// buildVerifier constructs the Clerk session verifier from env. It returns (nil, name) when
// Clerk is not configured *safely*, where name is the offending variable, so the caller can
// fail closed rather than expose unauthenticated — or unrestricted — content-source
// endpoints. It never returns or logs secret values.
//
// A verifier is built only when:
//   - CLERK_SECRET_KEY is present and non-blank, and
//   - CLERK_JWT_ISSUER is present and non-blank (issuer is mandatory — no unpinned issuer), and
//   - CLERK_AUTHORIZED_PARTIES is either absent (→ dev-default local origin only) or present
//     with at least one non-blank origin after trimming. A present-but-blank value is
//     treated as invalid configuration (never a silently unrestricted azp check).
func buildVerifier(lookup lookupEnv) (auth.Verifier, string) {
	secret, _ := lookup("CLERK_SECRET_KEY")
	if strings.TrimSpace(secret) == "" {
		return nil, "CLERK_SECRET_KEY"
	}

	issuer, _ := lookup("CLERK_JWT_ISSUER")
	if strings.TrimSpace(issuer) == "" {
		return nil, "CLERK_JWT_ISSUER"
	}

	parties, ok := authorizedParties(lookup)
	if !ok {
		return nil, "CLERK_AUTHORIZED_PARTIES"
	}

	return auth.NewClerkVerifier(auth.ClerkConfig{
		SecretKey:         secret,
		Issuer:            issuer,
		AuthorizedParties: parties,
	}), ""
}

// authorizedParties resolves the accepted `azp` origins. When CLERK_AUTHORIZED_PARTIES is
// absent it returns the local dev default only. When it is present but resolves to zero valid
// origins after trimming it returns ok=false so the caller fails closed — a present-but-blank
// value must never disable the azp check.
func authorizedParties(lookup lookupEnv) ([]string, bool) {
	raw, present := lookup("CLERK_AUTHORIZED_PARTIES")
	if !present {
		return []string{defaultAuthorizedParty}, true
	}
	parties := splitAndTrim(raw)
	if len(parties) == 0 {
		return nil, false
	}
	return parties, true
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
	verifier, missing := buildVerifier(os.LookupEnv)
	switch {
	case dsn == "":
		log.Println("DATABASE_URL not set: content-sources endpoints are disabled")
	case verifier == nil:
		// Fail closed: never mount content-source endpoints without a valid session verifier.
		// Log only the offending variable name — never a secret value.
		log.Printf("Clerk not configured (%s missing or invalid): content-sources endpoints stay disabled", missing)
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
