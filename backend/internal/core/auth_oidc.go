package core

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

type OIDCUserProvider struct {
	IssuerURL        string
	Audience         string
	UserIDClaim      string
	UsernameClaim    string
	DisplayNameClaim string
	GroupsClaim      string
	KeySet           *jwkSetCache
	Now              func() time.Time
}

type jwkSetCache struct {
	JWKSURL   string
	Client    *http.Client
	CacheTTL  time.Duration
	mu        sync.RWMutex
	keys      map[string]crypto.PublicKey
	fetchedAt time.Time
}

type oidcDiscoveryDocument struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

type jwksDocument struct {
	Keys []jsonWebKey `json:"keys"`
}

type jsonWebKey struct {
	KeyID string `json:"kid"`
	Type  string `json:"kty"`
	Use   string `json:"use"`
	Curve string `json:"crv"`
	N     string `json:"n"`
	E     string `json:"e"`
	X     string `json:"x"`
	Y     string `json:"y"`
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

func NewOIDCUserProvider(cfg Config) (OIDCUserProvider, error) {
	issuerURL := strings.TrimSpace(cfg.OIDCIssuerURL)
	if issuerURL == "" {
		return OIDCUserProvider{}, fmt.Errorf("AODS_OIDC_ISSUER_URL is required when AODS_AUTH_MODE=oidc")
	}

	client := &http.Client{Timeout: effectiveDuration(cfg.OIDCRequestTimeout, 5*time.Second)}
	jwksURL := strings.TrimSpace(cfg.OIDCJWKSURL)
	if jwksURL == "" {
		discovered, err := discoverOIDCJWKSURL(context.Background(), client, issuerURL)
		if err != nil {
			return OIDCUserProvider{}, err
		}
		jwksURL = discovered
	}

	return OIDCUserProvider{
		IssuerURL:        issuerURL,
		Audience:         strings.TrimSpace(cfg.OIDCAudience),
		UserIDClaim:      strings.TrimSpace(cfg.OIDCUserIDClaim),
		UsernameClaim:    strings.TrimSpace(cfg.OIDCUsernameClaim),
		DisplayNameClaim: strings.TrimSpace(cfg.OIDCDisplayNameClaim),
		GroupsClaim:      strings.TrimSpace(cfg.OIDCGroupsClaim),
		KeySet: &jwkSetCache{
			JWKSURL:  jwksURL,
			Client:   client,
			CacheTTL: 5 * time.Minute,
		},
		Now: time.Now,
	}, nil
}

func (p OIDCUserProvider) CurrentUser(r *http.Request) (User, error) {
	token, ok := bearerTokenFromRequest(r)
	if !ok {
		return User{}, ErrUnauthorized
	}

	claims, err := p.verifyToken(r.Context(), token)
	if err != nil {
		return User{}, err
	}

	userID := strings.TrimSpace(claimString(claimValue(claims, fallbackClaim(p.UserIDClaim, "sub"))))
	username := strings.TrimSpace(claimString(claimValue(claims, fallbackClaim(p.UsernameClaim, "preferred_username"))))
	displayName := strings.TrimSpace(claimString(claimValue(claims, fallbackClaim(p.DisplayNameClaim, "name"))))
	if displayName == "" {
		displayName = username
	}
	if userID == "" || username == "" {
		return User{}, ErrUnauthorized
	}

	return User{
		ID:          userID,
		Username:    username,
		DisplayName: displayName,
		Groups:      collectJWTGroups(claims, fallbackClaim(p.GroupsClaim, "groups")),
	}, nil
}

func (p OIDCUserProvider) verifyToken(ctx context.Context, token string) (map[string]any, error) {
	signingInput, header, claims, signature, err := parseJWT(token)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(header.Algorithm) == "" || strings.EqualFold(header.Algorithm, "none") {
		return nil, ErrUnauthorized
	}

	key, err := p.KeySet.PublicKey(ctx, header.KeyID)
	if err != nil {
		return nil, fmt.Errorf("fetch oidc jwks: %w", err)
	}
	if err := verifyJWTSignature(signingInput, signature, header.Algorithm, key); err != nil {
		return nil, ErrUnauthorized
	}
	if err := p.validateClaims(claims); err != nil {
		return nil, err
	}

	return claims, nil
}

func (p OIDCUserProvider) validateClaims(claims map[string]any) error {
	now := time.Now().UTC()
	if p.Now != nil {
		now = p.Now().UTC()
	}

	issuer := strings.TrimSpace(claimString(claimValue(claims, "iss")))
	if issuer == "" || issuer != strings.TrimSpace(p.IssuerURL) {
		return ErrUnauthorized
	}

	if !claimTimeAfter(claimValue(claims, "exp"), now) {
		return ErrUnauthorized
	}
	if !claimTimeReached(claimValue(claims, "nbf"), now) {
		return ErrUnauthorized
	}

	audience := strings.TrimSpace(p.Audience)
	if audience != "" && !claimMatchesAudience(claimValue(claims, "aud"), audience) {
		return ErrUnauthorized
	}

	return nil
}

func discoverOIDCJWKSURL(ctx context.Context, client *http.Client, issuerURL string) (string, error) {
	discoveryURL := strings.TrimRight(strings.TrimSpace(issuerURL), "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return "", fmt.Errorf("build oidc discovery request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request oidc discovery: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("oidc discovery returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var document oidcDiscoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&document); err != nil {
		return "", fmt.Errorf("decode oidc discovery: %w", err)
	}
	if strings.TrimSpace(document.JWKSURI) == "" {
		return "", fmt.Errorf("oidc discovery did not expose jwks_uri")
	}

	return strings.TrimSpace(document.JWKSURI), nil
}

func (c *jwkSetCache) PublicKey(ctx context.Context, keyID string) (crypto.PublicKey, error) {
	keys, err := c.keysForVerification(ctx, false)
	if err != nil {
		return nil, err
	}

	if key, ok := pickJWK(keys, keyID); ok {
		return key, nil
	}

	keys, err = c.keysForVerification(ctx, true)
	if err != nil {
		return nil, err
	}
	if key, ok := pickJWK(keys, keyID); ok {
		return key, nil
	}

	return nil, fmt.Errorf("oidc jwks does not contain key %q", keyID)
}

func (c *jwkSetCache) keysForVerification(ctx context.Context, forceRefresh bool) (map[string]crypto.PublicKey, error) {
	c.mu.RLock()
	if !forceRefresh && len(c.keys) > 0 && time.Since(c.fetchedAt) < c.cacheTTL() {
		keys := cloneKeyMap(c.keys)
		c.mu.RUnlock()
		return keys, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if !forceRefresh && len(c.keys) > 0 && time.Since(c.fetchedAt) < c.cacheTTL() {
		return cloneKeyMap(c.keys), nil
	}

	keys, err := c.fetchKeys(ctx)
	if err != nil {
		return nil, err
	}
	c.keys = keys
	c.fetchedAt = time.Now()
	return cloneKeyMap(c.keys), nil
}

func (c *jwkSetCache) fetchKeys(ctx context.Context) (map[string]crypto.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.JWKSURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build jwks request: %w", err)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("request jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("jwks returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var document jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&document); err != nil {
		return nil, fmt.Errorf("decode jwks: %w", err)
	}

	keys := make(map[string]crypto.PublicKey, len(document.Keys))
	for _, item := range document.Keys {
		key, err := item.publicKey()
		if err != nil {
			continue
		}
		keys[strings.TrimSpace(item.KeyID)] = key
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("jwks did not expose any usable public keys")
	}
	return keys, nil
}

func (c *jwkSetCache) cacheTTL() time.Duration {
	if c.CacheTTL <= 0 {
		return 5 * time.Minute
	}
	return c.CacheTTL
}

func (c *jwkSetCache) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return http.DefaultClient
}

func (k jsonWebKey) publicKey() (crypto.PublicKey, error) {
	switch strings.ToUpper(strings.TrimSpace(k.Type)) {
	case "RSA":
		return k.rsaPublicKey()
	case "EC":
		return k.ecPublicKey()
	default:
		return nil, fmt.Errorf("unsupported jwk kty %q", k.Type)
	}
}

func (k jsonWebKey) rsaPublicKey() (*rsa.PublicKey, error) {
	modulus, err := decodeJWTBigInt(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode jwk modulus: %w", err)
	}
	exponentBig, err := decodeJWTBigInt(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode jwk exponent: %w", err)
	}
	if !exponentBig.IsInt64() {
		return nil, fmt.Errorf("jwk exponent is too large")
	}
	return &rsa.PublicKey{
		N: modulus,
		E: int(exponentBig.Int64()),
	}, nil
}

func (k jsonWebKey) ecPublicKey() (*ecdsa.PublicKey, error) {
	curve, err := namedCurve(k.Curve)
	if err != nil {
		return nil, err
	}
	x, err := decodeJWTBigInt(k.X)
	if err != nil {
		return nil, fmt.Errorf("decode jwk x coordinate: %w", err)
	}
	y, err := decodeJWTBigInt(k.Y)
	if err != nil {
		return nil, fmt.Errorf("decode jwk y coordinate: %w", err)
	}
	if !curve.IsOnCurve(x, y) {
		return nil, fmt.Errorf("jwk point is not on the declared curve")
	}
	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
}

func namedCurve(value string) (elliptic.Curve, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "P-256":
		return elliptic.P256(), nil
	case "P-384":
		return elliptic.P384(), nil
	case "P-521":
		return elliptic.P521(), nil
	default:
		return nil, fmt.Errorf("unsupported jwk curve %q", value)
	}
}

func parseJWT(token string) (string, jwtHeader, map[string]any, []byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", jwtHeader{}, nil, nil, ErrUnauthorized
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", jwtHeader{}, nil, nil, ErrUnauthorized
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", jwtHeader{}, nil, nil, ErrUnauthorized
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", jwtHeader{}, nil, nil, ErrUnauthorized
	}

	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return "", jwtHeader{}, nil, nil, ErrUnauthorized
	}
	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return "", jwtHeader{}, nil, nil, ErrUnauthorized
	}

	return parts[0] + "." + parts[1], header, claims, signature, nil
}

func verifyJWTSignature(signingInput string, signature []byte, algorithm string, key crypto.PublicKey) error {
	digest, hash, err := hashJWTSigningInput(signingInput, algorithm)
	if err != nil {
		return err
	}

	switch publicKey := key.(type) {
	case *rsa.PublicKey:
		switch algorithm {
		case "RS256", "RS384", "RS512":
			return rsa.VerifyPKCS1v15(publicKey, hash, digest, signature)
		case "PS256", "PS384", "PS512":
			return rsa.VerifyPSS(publicKey, hash, digest, signature, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
		default:
			return ErrUnauthorized
		}
	case *ecdsa.PublicKey:
		switch algorithm {
		case "ES256", "ES384", "ES512":
			if !ecdsa.VerifyASN1(publicKey, digest, signature) {
				return ErrUnauthorized
			}
			return nil
		default:
			return ErrUnauthorized
		}
	default:
		return ErrUnauthorized
	}
}

func hashJWTSigningInput(signingInput string, algorithm string) ([]byte, crypto.Hash, error) {
	switch algorithm {
	case "RS256", "PS256", "ES256":
		sum := sha256.Sum256([]byte(signingInput))
		return sum[:], crypto.SHA256, nil
	case "RS384", "PS384", "ES384":
		sum := sha512.Sum384([]byte(signingInput))
		return sum[:], crypto.SHA384, nil
	case "RS512", "PS512", "ES512":
		sum := sha512.Sum512([]byte(signingInput))
		return sum[:], crypto.SHA512, nil
	default:
		return nil, 0, ErrUnauthorized
	}
}

func claimValue(claims map[string]any, path string) any {
	current := any(claims)
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		if part == "" {
			return nil
		}
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = object[part]
	}
	return current
}

func claimString(value any) string {
	text, _ := value.(string)
	return text
}

func claimStrings(value any) []string {
	switch typed := value.(type) {
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				items = append(items, strings.TrimSpace(text))
			}
		}
		return items
	case []string:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			if strings.TrimSpace(item) != "" {
				items = append(items, strings.TrimSpace(item))
			}
		}
		return items
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []string{strings.TrimSpace(typed)}
	default:
		return nil
	}
}

func collectJWTGroups(claims map[string]any, groupsClaim string) []string {
	groups := claimStrings(claimValue(claims, groupsClaim))
	groups = append(groups, claimStrings(claimValue(claims, "realm_access.roles"))...)

	if resourceAccess, ok := claimValue(claims, "resource_access").(map[string]any); ok {
		for _, raw := range resourceAccess {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			groups = append(groups, claimStrings(entry["roles"])...)
		}
	}

	seen := make(map[string]struct{}, len(groups))
	items := make([]string, 0, len(groups))
	for _, group := range groups {
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		items = append(items, group)
	}
	return items
}

func claimTimeAfter(value any, now time.Time) bool {
	seconds, ok := claimNumericDate(value)
	return ok && now.Before(time.Unix(seconds, 0))
}

func claimTimeReached(value any, now time.Time) bool {
	if value == nil {
		return true
	}
	seconds, ok := claimNumericDate(value)
	return ok && !now.Before(time.Unix(seconds, 0))
}

func claimNumericDate(value any) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case int64:
		return typed, true
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func claimMatchesAudience(value any, expected string) bool {
	switch typed := value.(type) {
	case string:
		return typed == expected
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok && text == expected {
				return true
			}
		}
	case []string:
		for _, item := range typed {
			if item == expected {
				return true
			}
		}
	}
	return false
}

func decodeJWTBigInt(value string) (*big.Int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, err
	}
	integer := new(big.Int).SetBytes(raw)
	if integer.Sign() == 0 {
		return nil, fmt.Errorf("value is zero")
	}
	return integer, nil
}

func pickJWK(keys map[string]crypto.PublicKey, keyID string) (crypto.PublicKey, bool) {
	if keyID != "" {
		key, ok := keys[keyID]
		return key, ok
	}
	if len(keys) != 1 {
		return nil, false
	}
	for _, key := range keys {
		return key, true
	}
	return nil, false
}

func cloneKeyMap(source map[string]crypto.PublicKey) map[string]crypto.PublicKey {
	cloned := make(map[string]crypto.PublicKey, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func fallbackClaim(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func effectiveDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}
