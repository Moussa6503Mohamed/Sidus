package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	healthz(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}

// envMap builds a lookupEnv over a fixed map, distinguishing absent keys (missing entry)
// from present-but-empty keys (entry with an empty value).
func envMap(m map[string]string) lookupEnv {
	return func(key string) (string, bool) {
		v, ok := m[key]
		return v, ok
	}
}

// TestBuildVerifier_RequiresSecretAndIssuer proves content-source routes fail closed unless
// both the secret and the (mandatory) issuer are present and non-blank. The reported
// variable name lets main log which config is missing without ever logging a secret value.
func TestBuildVerifier_RequiresSecretAndIssuer(t *testing.T) {
	cases := map[string]struct {
		env         map[string]string
		wantMissing string
	}{
		"no secret": {
			map[string]string{"CLERK_JWT_ISSUER": "https://x.clerk.accounts.dev"},
			"CLERK_SECRET_KEY",
		},
		"blank secret": {
			map[string]string{"CLERK_SECRET_KEY": "   ", "CLERK_JWT_ISSUER": "https://x.clerk.accounts.dev"},
			"CLERK_SECRET_KEY",
		},
		"no issuer": {
			map[string]string{"CLERK_SECRET_KEY": "sk_test_x"},
			"CLERK_JWT_ISSUER",
		},
		"blank issuer": {
			map[string]string{"CLERK_SECRET_KEY": "sk_test_x", "CLERK_JWT_ISSUER": "  "},
			"CLERK_JWT_ISSUER",
		},
		"whitespace-only issuer with tab": {
			map[string]string{"CLERK_SECRET_KEY": "sk_test_x", "CLERK_JWT_ISSUER": "\t"},
			"CLERK_JWT_ISSUER",
		},
	}
	for name, c := range cases {
		v, missing := buildVerifier(envMap(c.env))
		if v != nil {
			t.Fatalf("%s: verifier = non-nil, want nil (fail closed)", name)
		}
		if missing != c.wantMissing {
			t.Fatalf("%s: missing = %q, want %q", name, missing, c.wantMissing)
		}
	}
}

// TestBuildVerifier_AuthorizedPartiesDefaultAndFailClosed proves the azp allow-list never
// becomes silently unrestricted: absent → dev-default local origin only; present-but-blank →
// invalid config (no verifier).
func TestBuildVerifier_AuthorizedPartiesDefaultAndFailClosed(t *testing.T) {
	base := map[string]string{
		"CLERK_SECRET_KEY": "sk_test_x",
		"CLERK_JWT_ISSUER": "https://x.clerk.accounts.dev",
	}

	// Absent → verifier built with the local dev default only.
	if v, missing := buildVerifier(envMap(base)); v == nil {
		t.Fatalf("absent authorized parties: verifier = nil (missing=%q), want built with dev default", missing)
	}

	// Present but blank/whitespace/commas-only → zero valid origins → fail closed.
	for _, blank := range []string{"", "   ", " , ,  ", ","} {
		env := map[string]string{
			"CLERK_SECRET_KEY":         "sk_test_x",
			"CLERK_JWT_ISSUER":         "https://x.clerk.accounts.dev",
			"CLERK_AUTHORIZED_PARTIES": blank,
		}
		v, missing := buildVerifier(envMap(env))
		if v != nil {
			t.Fatalf("blank authorized parties %q: verifier = non-nil, want nil (fail closed)", blank)
		}
		if missing != "CLERK_AUTHORIZED_PARTIES" {
			t.Fatalf("blank authorized parties %q: missing = %q, want CLERK_AUTHORIZED_PARTIES", blank, missing)
		}
	}
}

// TestBuildVerifier_FullyConfigured proves a complete, safe configuration yields a verifier.
func TestBuildVerifier_FullyConfigured(t *testing.T) {
	env := map[string]string{
		"CLERK_SECRET_KEY":         "sk_test_x",
		"CLERK_JWT_ISSUER":         "https://x.clerk.accounts.dev",
		"CLERK_AUTHORIZED_PARTIES": "https://app.sidus.example, https://admin.sidus.example",
	}
	v, missing := buildVerifier(envMap(env))
	if v == nil {
		t.Fatalf("fully configured: verifier = nil (missing=%q), want non-nil", missing)
	}
	if missing != "" {
		t.Fatalf("fully configured: missing = %q, want empty", missing)
	}
}

// TestAuthorizedParties_Absent proves an absent env var yields only the local dev default.
func TestAuthorizedParties_Absent(t *testing.T) {
	parties, ok := authorizedParties(envMap(map[string]string{}))
	if !ok {
		t.Fatal("absent: ok = false, want true (dev default)")
	}
	if len(parties) != 1 || parties[0] != defaultAuthorizedParty {
		t.Fatalf("absent: parties = %v, want [%q]", parties, defaultAuthorizedParty)
	}
}
