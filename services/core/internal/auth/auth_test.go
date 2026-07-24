package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseRole(t *testing.T) {
	cases := map[string]Role{
		"learner":  RoleLearner,
		"editor":   RoleEditor,
		"reviewer": RoleReviewer,
		"admin":    RoleAdmin,
		" admin ":  RoleAdmin,
		"":         RoleUnknown,
		"root":     RoleUnknown,
		"Admin":    RoleUnknown, // case-sensitive: only exact known values are honored
	}
	for raw, want := range cases {
		if got := ParseRole(raw); got != want {
			t.Fatalf("ParseRole(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestRolePermissionMatrix(t *testing.T) {
	all := []Permission{PermReadSource, PermCreateSource, PermUpdateSource, PermReviewSource}

	// learner and unknown: no content-source permission at all.
	for _, r := range []Role{RoleLearner, RoleUnknown} {
		for _, p := range all {
			if r.Can(p) {
				t.Fatalf("%q must not have %q", r, p)
			}
		}
	}

	// editor: read/create/update, but not review.
	for _, p := range []Permission{PermReadSource, PermCreateSource, PermUpdateSource} {
		if !RoleEditor.Can(p) {
			t.Fatalf("editor must have %q", p)
		}
	}
	if RoleEditor.Can(PermReviewSource) {
		t.Fatal("editor must not have review")
	}

	// reviewer and admin: all permissions.
	for _, r := range []Role{RoleReviewer, RoleAdmin} {
		for _, p := range all {
			if !r.Can(p) {
				t.Fatalf("%q must have %q", r, p)
			}
		}
	}
}

// stubVerifier maps a fixed token to fixed claims for middleware tests.
type stubVerifier struct {
	token  string
	claims Claims
}

func (s stubVerifier) Verify(_ context.Context, token string) (Claims, error) {
	if token == s.token {
		return s.claims, nil
	}
	return Claims{}, ErrInvalidToken
}

func protectedHandler(t *testing.T, wantSubject string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			t.Fatal("claims missing from context in protected handler")
		}
		if claims.Subject != wantSubject {
			t.Fatalf("subject = %q, want %q", claims.Subject, wantSubject)
		}
		w.WriteHeader(http.StatusOK)
	}
}

func TestProtect_MissingToken_401(t *testing.T) {
	v := stubVerifier{token: "t", claims: Claims{Subject: "u", Role: RoleAdmin}}
	h := Protect(v, PermReadSource, protectedHandler(t, "u"))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "missing_token" {
		t.Fatalf("error = %q, want missing_token", body["error"])
	}
}

func TestProtect_InvalidToken_401(t *testing.T) {
	v := stubVerifier{token: "good", claims: Claims{Subject: "u", Role: RoleAdmin}}
	h := Protect(v, PermReadSource, protectedHandler(t, "u"))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer bad")
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestProtect_Forbidden_403(t *testing.T) {
	v := stubVerifier{token: "t", claims: Claims{Subject: "u", Role: RoleLearner}}
	h := Protect(v, PermReadSource, protectedHandler(t, "u"))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer t")
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestProtect_Success_InjectsClaims(t *testing.T) {
	v := stubVerifier{token: "t", claims: Claims{Subject: "user_42", Role: RoleAdmin}}
	h := Protect(v, PermReviewSource, protectedHandler(t, "user_42"))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer t")
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestBearerToken(t *testing.T) {
	cases := map[string]struct {
		header    string
		wantToken string
		wantOK    bool
	}{
		"valid":         {"Bearer abc.def.ghi", "abc.def.ghi", true},
		"case-insens":   {"bearer abc", "abc", true},
		"missing":       {"", "", false},
		"no-prefix":     {"abc", "", false},
		"empty-token":   {"Bearer ", "", false},
		"spaces-only":   {"Bearer    ", "", false},
		"wrong-scheme":  {"Basic abc", "", false},
	}
	for name, c := range cases {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		if c.header != "" {
			req.Header.Set("Authorization", c.header)
		}
		got, ok := bearerToken(req)
		if ok != c.wantOK || got != c.wantToken {
			t.Fatalf("%s: bearerToken = (%q, %v), want (%q, %v)", name, got, ok, c.wantToken, c.wantOK)
		}
	}
}
