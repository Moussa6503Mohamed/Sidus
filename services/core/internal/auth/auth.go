// Package auth holds Sidus Core's authorization layer. Clerk owns authentication (issuing
// and signing session JWTs); Sidus Core owns authorization: it verifies the Clerk session
// token, derives the authenticated actor from the verified subject (never from the request
// body), and maps the verified `sidus_role` claim to a fixed permission set.
//
// The Verifier interface isolates token verification so HTTP handlers and the role matrix
// can be tested without any live Clerk instance or cryptography; the production
// implementation lives in clerk_verifier.go.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Role is a Sidus authorization role, sourced from the verified `sidus_role` session claim.
type Role string

const (
	// RoleLearner has no content-source access.
	RoleLearner Role = "learner"
	// RoleEditor may create and update (PATCH) pending sources.
	RoleEditor Role = "editor"
	// RoleReviewer has editor permissions plus approve/reject.
	RoleReviewer Role = "reviewer"
	// RoleAdmin has all content-source permissions.
	RoleAdmin Role = "admin"
	// RoleUnknown is any missing or unrecognized role: denied by default.
	RoleUnknown Role = ""
)

// Permission is a discrete capability guarded on an endpoint.
type Permission string

const (
	// PermReadSource covers listing and reading content sources.
	PermReadSource Permission = "content_source:read"
	// PermCreateSource covers creating a pending content source.
	PermCreateSource Permission = "content_source:create"
	// PermUpdateSource covers PATCHing a pending content source's metadata.
	PermUpdateSource Permission = "content_source:update"
	// PermReviewSource covers approving and rejecting a content source.
	PermReviewSource Permission = "content_source:review"
)

// rolePermissions is the least-privilege matrix. A role absent from this map, or the
// RoleUnknown value, has no permissions and is denied by default.
var rolePermissions = map[Role]map[Permission]bool{
	RoleLearner: {},
	RoleEditor: {
		PermReadSource:   true,
		PermCreateSource: true,
		PermUpdateSource: true,
	},
	RoleReviewer: {
		PermReadSource:   true,
		PermCreateSource: true,
		PermUpdateSource: true,
		PermReviewSource: true,
	},
	RoleAdmin: {
		PermReadSource:   true,
		PermCreateSource: true,
		PermUpdateSource: true,
		PermReviewSource: true,
	},
}

// ParseRole maps a raw `sidus_role` claim value to a known Role, or RoleUnknown if the
// value is missing or unrecognized. Deny-by-default: unknown roles carry no permissions.
func ParseRole(raw string) Role {
	switch Role(strings.TrimSpace(raw)) {
	case RoleLearner:
		return RoleLearner
	case RoleEditor:
		return RoleEditor
	case RoleReviewer:
		return RoleReviewer
	case RoleAdmin:
		return RoleAdmin
	default:
		return RoleUnknown
	}
}

// Can reports whether the role is granted the permission.
func (r Role) Can(p Permission) bool {
	return rolePermissions[r][p]
}

// Claims is the minimal verified identity Sidus Core trusts: the Clerk subject (used as the
// audit actor/reviewer) and the authorization role. It is produced only by a Verifier from
// a cryptographically verified token — never from a request body.
type Claims struct {
	Subject string
	Role    Role
}

// ErrInvalidToken is returned by a Verifier when a token is missing, malformed, expired,
// has a bad signature, or fails issuer/authorized-party checks. The caller maps it to 401.
var ErrInvalidToken = errors.New("invalid or missing session token")

// Verifier verifies a raw bearer token and returns the verified Claims.
type Verifier interface {
	Verify(ctx context.Context, token string) (Claims, error)
}

type contextKey struct{}

// ContextWithClaims returns a copy of ctx carrying the verified claims.
func ContextWithClaims(ctx context.Context, c Claims) context.Context {
	return context.WithValue(ctx, contextKey{}, c)
}

// ClaimsFromContext returns the verified claims placed on the context by Protect.
func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(contextKey{}).(Claims)
	return c, ok
}

// Protect wraps next so it runs only for a request bearing a valid Clerk session token
// whose role holds perm. Missing/invalid token -> 401; valid token lacking the permission
// -> 403. On success the verified Claims are attached to the request context so the handler
// can read the authenticated subject.
func Protect(v Verifier, perm Permission, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			writeAuthError(w, http.StatusUnauthorized, "missing_token", "a Clerk session bearer token is required")
			return
		}
		claims, err := v.Verify(r.Context(), token)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "invalid_token", "session token is missing, invalid, or expired")
			return
		}
		if !claims.Role.Can(perm) {
			writeAuthError(w, http.StatusForbidden, "forbidden", "your role does not permit this action")
			return
		}
		next(w, r.WithContext(ContextWithClaims(r.Context(), claims)))
	}
}

// bearerToken extracts the token from an `Authorization: Bearer <token>` header.
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(h[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}

func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message})
}
