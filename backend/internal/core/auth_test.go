package core

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type staticUserProvider struct {
	User User
	Err  error
}

func (p staticUserProvider) CurrentUser(r *http.Request) (User, error) {
	if p.Err != nil {
		return User{}, p.Err
	}
	return p.User, nil
}

func TestHeaderUserProviderFallsBackToDevUser(t *testing.T) {
	t.Parallel()

	provider := HeaderUserProvider{
		AllowDevFallback: true,
		DevUser: User{
			ID:       "local-user",
			Username: "local.developer",
		},
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	user, err := provider.CurrentUser(request)
	if err != nil {
		t.Fatalf("current user: %v", err)
	}
	if user.Username != "local.developer" {
		t.Fatalf("unexpected dev fallback user %#v", user)
	}
}

func TestOIDCUserProviderCurrentUser(t *testing.T) {
	t.Parallel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	var issuerURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":   issuerURL,
				"jwks_uri": issuerURL + "/jwks",
			})
		case "/jwks":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"keys": []map[string]any{
					{
						"kty": "RSA",
						"kid": "primary",
						"n":   base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes()),
						"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.E)).Bytes()),
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	issuerURL = server.URL

	provider, err := NewOIDCUserProvider(Config{
		AuthMode:             "oidc",
		OIDCIssuerURL:        issuerURL,
		OIDCAudience:         "aods-portal",
		OIDCRequestTimeout:   2 * time.Second,
		OIDCGroupsClaim:      "groups",
		OIDCUsernameClaim:    "preferred_username",
		OIDCDisplayNameClaim: "name",
	})
	if err != nil {
		t.Fatalf("new oidc provider: %v", err)
	}
	provider.Now = func() time.Time {
		return time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	}

	token := signRS256JWT(t, privateKey, "primary", map[string]any{
		"iss":                issuerURL,
		"aud":                "aods-portal",
		"sub":                "user-1",
		"preferred_username": "alice",
		"name":               "Alice",
		"groups":             []string{"aods:project-a:deploy"},
		"realm_access": map[string]any{
			"roles": []string{"aods:platform:admin"},
		},
		"exp": provider.Now().Add(time.Hour).Unix(),
		"nbf": provider.Now().Add(-time.Minute).Unix(),
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	user, err := provider.CurrentUser(request)
	if err != nil {
		t.Fatalf("current user: %v", err)
	}
	if user.ID != "user-1" || user.Username != "alice" {
		t.Fatalf("unexpected user %#v", user)
	}
	if len(user.Groups) != 2 {
		t.Fatalf("expected merged groups, got %#v", user.Groups)
	}
}

func TestOIDCUserProviderCollectsClientQualifiedRoles(t *testing.T) {
	t.Parallel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	var issuerURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":   issuerURL,
				"jwks_uri": issuerURL + "/jwks",
			})
		case "/jwks":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"keys": []map[string]any{
					{
						"kty": "RSA",
						"kid": "primary",
						"n":   base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes()),
						"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.E)).Bytes()),
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	issuerURL = server.URL

	provider, err := NewOIDCUserProvider(Config{
		AuthMode:             "oidc",
		OIDCIssuerURL:        issuerURL,
		OIDCAudience:         "aods-portal",
		OIDCRequestTimeout:   2 * time.Second,
		OIDCGroupsClaim:      "groups",
		OIDCUsernameClaim:    "preferred_username",
		OIDCDisplayNameClaim: "name",
	})
	if err != nil {
		t.Fatalf("new oidc provider: %v", err)
	}
	provider.Now = func() time.Time {
		return time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC)
	}

	token := signRS256JWT(t, privateKey, "primary", map[string]any{
		"iss":                issuerURL,
		"aud":                "aods-portal",
		"sub":                "user-1",
		"preferred_username": "alice",
		"name":               "Alice",
		"resource_access": map[string]any{
			"account": map[string]any{
				"roles": []string{"manage-account"},
			},
		},
		"exp": provider.Now().Add(time.Hour).Unix(),
		"nbf": provider.Now().Add(-time.Minute).Unix(),
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	user, err := provider.CurrentUser(request)
	if err != nil {
		t.Fatalf("current user: %v", err)
	}

	expected := []string{"manage-account", "account:manage-account"}
	if len(user.Groups) != len(expected) {
		t.Fatalf("unexpected groups %#v", user.Groups)
	}
	for index, value := range expected {
		if user.Groups[index] != value {
			t.Fatalf("unexpected groups %#v", user.Groups)
		}
	}
}

func TestAuthorityMappingUserProviderExpandsCanonicalRoles(t *testing.T) {
	t.Parallel()

	provider := AuthorityMappingUserProvider{
		Base: staticUserProvider{
			User: User{
				ID:       "user-1",
				Username: "alice",
				Groups:   []string{"kc-platform-admin", "kc-project-a-deployer"},
			},
		},
		Mappings: map[string][]string{
			"kc-platform-admin":     {"aods:platform:admin"},
			"kc-project-a-deployer": {"aods:project-a:deploy"},
		},
	}

	user, err := provider.CurrentUser(httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
	if err != nil {
		t.Fatalf("current user: %v", err)
	}

	expected := []string{
		"kc-platform-admin",
		"aods:platform:admin",
		"kc-project-a-deployer",
		"aods:project-a:deploy",
	}
	if len(user.Groups) != len(expected) {
		t.Fatalf("unexpected mapped groups %#v", user.Groups)
	}
	for index, value := range expected {
		if user.Groups[index] != value {
			t.Fatalf("unexpected mapped groups %#v", user.Groups)
		}
	}
}

func TestParseAuthorityMappings(t *testing.T) {
	t.Parallel()

	mappings, err := parseAuthorityMappings("kc-platform-admin=aods:platform:admin,kc-project-a-deployer=aods:project-a:deploy|aods:project-a:view")
	if err != nil {
		t.Fatalf("parse authority mappings: %v", err)
	}

	if got := mappings["kc-platform-admin"]; len(got) != 1 || got[0] != "aods:platform:admin" {
		t.Fatalf("unexpected platform mapping %#v", got)
	}
	if got := mappings["kc-project-a-deployer"]; len(got) != 2 || got[0] != "aods:project-a:deploy" || got[1] != "aods:project-a:view" {
		t.Fatalf("unexpected deployer mapping %#v", got)
	}
}

func TestParseAuthorityMappingsRejectsInvalidEntries(t *testing.T) {
	t.Parallel()

	if _, err := parseAuthorityMappings("broken-entry"); err == nil {
		t.Fatal("expected invalid authority mapping to fail")
	}
}

func TestCompositeUserProviderRejectsInvalidBearerBeforeHeaderFallback(t *testing.T) {
	t.Parallel()

	provider := CompositeUserProvider{
		Primary: ErrorUserProvider{Err: ErrUnauthorized},
		DevFallback: DevFallbackUserProvider{
			AllowDevFallback: true,
			DevUser: User{
				ID:       "local-user",
				Username: "local.developer",
			},
		},
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer invalid")
	request.Header.Set("X-AODS-User-Id", "user-1")
	request.Header.Set("X-AODS-Username", "alice")

	_, err := provider.CurrentUser(request)
	if err == nil {
		t.Fatal("expected bearer auth failure")
	}
}

func TestCompositeUserProviderFallsBackWhenBearerIsAbsent(t *testing.T) {
	t.Parallel()

	provider := CompositeUserProvider{
		Primary: ErrorUserProvider{Err: ErrUnauthorized},
		DevFallback: DevFallbackUserProvider{
			AllowDevFallback: true,
			DevUser: User{
				ID:       "local-user",
				Username: "local.developer",
			},
		},
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)

	user, err := provider.CurrentUser(request)
	if err != nil {
		t.Fatalf("current user: %v", err)
	}
	if user.Username != "local.developer" {
		t.Fatalf("unexpected fallback user %#v", user)
	}
}

func signRS256JWT(t *testing.T, privateKey *rsa.PrivateKey, keyID string, claims map[string]any) string {
	t.Helper()

	headerBytes, err := json.Marshal(map[string]any{
		"alg": "RS256",
		"typ": "JWT",
		"kid": keyID,
	})
	if err != nil {
		t.Fatalf("marshal jwt header: %v", err)
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal jwt claims: %v", err)
	}

	header := base64.RawURLEncoding.EncodeToString(headerBytes)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingInput := header + "." + payload
	digest := sha256.Sum256([]byte(signingInput))

	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}
