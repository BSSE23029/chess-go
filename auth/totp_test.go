package auth

import (
	"bytes"
	"encoding/base32"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRFC6238EightDigitVectors(t *testing.T) {
	secret := []byte("12345678901234567890")
	vectors := []struct {
		unix int64
		code string
	}{
		{59, "94287082"},
		{1111111109, "07081804"},
		{1111111111, "14050471"},
		{1234567890, "89005924"},
		{2000000000, "69279037"},
		{20000000000, "65353130"},
	}
	for _, vector := range vectors {
		got, err := Generate(secret, time.Unix(vector.unix, 0))
		if err != nil || got != vector.code {
			t.Errorf("TOTP at %d = %q, %v; want %q", vector.unix, got, err, vector.code)
		}
	}
}

func TestVerifierRejectsReplayAndMalformedCodes(t *testing.T) {
	secret := []byte("12345678901234567890")
	now := time.Unix(1234567890, 0)
	code, err := Generate(secret, now)
	if err != nil {
		t.Fatal(err)
	}
	verifier := NewVerifier()
	if err := verifier.Verify("alice", secret, code, now, 1); err != nil {
		t.Fatal(err)
	}
	if err := verifier.Verify("alice", secret, code, now, 1); !errors.Is(err, ErrReplay) {
		t.Fatalf("replayed code error = %v", err)
	}
	if Verify(secret, "1234", now, 1) {
		t.Fatal("short code accepted")
	}
	if Verify(secret, code, now, 2) {
		t.Fatal("overly wide verification window accepted")
	}
}

func TestTOTPWindowAndProvisioningURI(t *testing.T) {
	secret := []byte("12345678901234567890")
	now := time.Unix(1234567890, 0)
	previous, err := Generate(secret, now.Add(-TimeStep))
	if err != nil {
		t.Fatal(err)
	}
	if !Verify(secret, previous, now, 1) || Verify(secret, previous, now, 0) {
		t.Fatal("clock-skew window behavior is incorrect")
	}
	uri, err := ProvisioningURI("Chess Go", "alice@example.test", secret)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"otpauth://totp/", "issuer=Chess+Go", "digits=8", "period=30", "secret=" + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)} {
		if !strings.Contains(uri, want) {
			t.Fatalf("provisioning URI %q lacks %q", uri, want)
		}
	}
}

func TestEncryptedSecretRoundTripAndTamperRejection(t *testing.T) {
	secret := []byte("12345678901234567890")
	key := bytes.Repeat([]byte{7}, 32)
	ciphertext, err := EncryptSecret(secret, key)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(ciphertext, secret) {
		t.Fatal("encrypted secret is plaintext")
	}
	decoded, err := DecryptSecret(ciphertext, key)
	if err != nil || !bytes.Equal(decoded, secret) {
		t.Fatalf("decrypted secret = %x, %v", decoded, err)
	}
	ciphertext[len(ciphertext)-1] ^= 1
	if _, err := DecryptSecret(ciphertext, key); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
	if _, err := EncryptSecret(secret, []byte("short")); err == nil {
		t.Fatal("short encryption key accepted")
	}
}
