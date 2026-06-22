package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testKey = "super-secret-test-key-32-bytes-min"

func TestMintAndVerify_RoundTrip(t *testing.T) {
	tok, err := Mint(testKey, "frontend-2-v2", "alice@infoblox.com", "Alice")
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	claims, err := Verify(tok, testKey)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Email != "alice@infoblox.com" {
		t.Errorf("Email = %q, want alice@infoblox.com", claims.Email)
	}
	if claims.Name != "Alice" {
		t.Errorf("Name = %q, want Alice", claims.Name)
	}
	if claims.Issuer != "frontend-2-v2" {
		t.Errorf("Issuer = %q, want frontend-2-v2", claims.Issuer)
	}
}

func TestVerify_WrongKey(t *testing.T) {
	tok, err := Mint(testKey, "frontend-2-v2", "alice@infoblox.com", "Alice")
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if _, err := Verify(tok, "different-key"); err == nil {
		t.Error("Verify with wrong key should fail")
	}
}

func TestVerify_MalformedToken(t *testing.T) {
	if _, err := Verify("not-a-jwt", testKey); err == nil {
		t.Error("Verify of malformed token should fail")
	}
}

func TestVerify_RejectsNoneAlg(t *testing.T) {
	claims := Claims{
		Email: "x@y.z",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	signed, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	if _, err := Verify(signed, testKey); err == nil {
		t.Error("Verify must reject alg=none tokens")
	}
}

func TestVerify_Expired(t *testing.T) {
	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Email: "alice@infoblox.com",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	})
	signed, err := expired.SignedString([]byte(testKey))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	if _, err := Verify(signed, testKey); err == nil {
		t.Error("Verify must reject expired tokens")
	}
}

func TestMint_TokenShape(t *testing.T) {
	tok, err := Mint(testKey, "frontend-2-v2", "a@b.c", "A B")
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if parts := strings.Split(tok, "."); len(parts) != 3 {
		t.Errorf("JWT should have 3 segments, got %d", len(parts))
	}
}
