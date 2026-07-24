package auth

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwks"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
)

// ClerkConfig configures the production Clerk verifier.
type ClerkConfig struct {
	// SecretKey is the Clerk Backend API secret (sk_...). Sourced from the environment;
	// never logged or embedded. Used only to authorize JWKS fetches.
	SecretKey string
	// Issuer, when set, pins the accepted token issuer (the Clerk Frontend API URL, e.g.
	// https://your-app.clerk.accounts.dev). Verification rejects any other issuer.
	Issuer string
	// AuthorizedParties is the allow-list of `azp` values (request origins) accepted on a
	// token. In development this must contain only the local Sidus origin.
	AuthorizedParties []string
	// JWKCacheTTL bounds how long a fetched signing key is reused before refetching, so key
	// rotation is eventually picked up without a Backend API call per request. Defaults to
	// one hour when zero.
	JWKCacheTTL time.Duration
}

// sidusCustomClaims captures the Sidus-specific session claim carrying the authorization
// role. It is the only custom claim Sidus reads.
type sidusCustomClaims struct {
	SidusRole string `json:"sidus_role"`
}

// ClerkVerifier verifies Clerk session JWTs using the official Clerk Go SDK's networkless
// JWKS verification. Signing keys are cached by key ID so a valid session does not incur a
// Clerk Backend API call on every request; only an unknown or expired key triggers a fetch.
type ClerkVerifier struct {
	jwksClient        *jwks.Client
	issuer            string
	authorizedParties []string
	cacheTTL          time.Duration

	mu    sync.RWMutex
	cache map[string]cachedJWK
}

type cachedJWK struct {
	jwk       *clerk.JSONWebKey
	expiresAt time.Time
}

const defaultJWKCacheTTL = time.Hour

// NewClerkVerifier builds a ClerkVerifier from config.
func NewClerkVerifier(cfg ClerkConfig) *ClerkVerifier {
	clientConfig := &clerk.ClientConfig{}
	clientConfig.Key = clerk.String(cfg.SecretKey)
	ttl := cfg.JWKCacheTTL
	if ttl <= 0 {
		ttl = defaultJWKCacheTTL
	}
	return &ClerkVerifier{
		jwksClient:        jwks.NewClient(clientConfig),
		issuer:            strings.TrimSpace(cfg.Issuer),
		authorizedParties: cfg.AuthorizedParties,
		cacheTTL:          ttl,
		cache:             map[string]cachedJWK{},
	}
}

// Verify implements Verifier. It resolves the token's signing key (from cache when possible),
// verifies signature/expiry/issuer/authorized-party via the Clerk SDK, then extracts the
// verified subject and `sidus_role`.
func (v *ClerkVerifier) Verify(ctx context.Context, token string) (Claims, error) {
	unsafe, err := jwt.Decode(ctx, &jwt.DecodeParams{Token: token})
	if err != nil {
		return Claims{}, ErrInvalidToken
	}

	jwk, err := v.signingKey(ctx, unsafe.KeyID)
	if err != nil {
		return Claims{}, ErrInvalidToken
	}

	custom := &sidusCustomClaims{}
	claims, err := jwt.Verify(ctx, &jwt.VerifyParams{
		Token: token,
		JWK:   jwk,
		CustomClaimsConstructor: func(_ context.Context) any {
			return custom
		},
		AuthorizedPartyHandler: v.authorizedParty,
	})
	if err != nil {
		return Claims{}, ErrInvalidToken
	}

	// The SDK validates the issuer is a well-formed Clerk issuer; additionally pin it to the
	// configured instance so tokens from any other Clerk instance are rejected.
	if v.issuer != "" && claims.Issuer != v.issuer {
		return Claims{}, ErrInvalidToken
	}
	if claims.Subject == "" {
		return Claims{}, ErrInvalidToken
	}

	return Claims{Subject: claims.Subject, Role: ParseRole(custom.SidusRole)}, nil
}

// authorizedParty is the SDK AuthorizedPartyHandler: the token's `azp` must be one of the
// configured origins. When no origins are configured the check is skipped (verification
// still enforces signature/issuer/expiry), but production/dev config always sets one.
func (v *ClerkVerifier) authorizedParty(azp string) bool {
	if len(v.authorizedParties) == 0 {
		return true
	}
	for _, p := range v.authorizedParties {
		if azp == p {
			return true
		}
	}
	return false
}

// signingKey returns the JWK for kid, using the cache when a fresh entry exists and fetching
// (and caching) from Clerk otherwise.
func (v *ClerkVerifier) signingKey(ctx context.Context, kid string) (*clerk.JSONWebKey, error) {
	if kid == "" {
		return nil, fmt.Errorf("token missing key id")
	}

	v.mu.RLock()
	entry, ok := v.cache[kid]
	v.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.jwk, nil
	}

	jwk, err := jwt.GetJSONWebKey(ctx, &jwt.GetJSONWebKeyParams{
		KeyID:      kid,
		JWKSClient: v.jwksClient,
	})
	if err != nil {
		return nil, err
	}

	v.mu.Lock()
	v.cache[kid] = cachedJWK{jwk: jwk, expiresAt: time.Now().Add(v.cacheTTL)}
	v.mu.Unlock()
	return jwk, nil
}
