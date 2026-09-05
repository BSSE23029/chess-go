// Package auth contains small, dependency-free authentication primitives for
// network integrations. It deliberately does not own accounts or sessions.
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// Digits is the fixed width requested by the chess-go authenticator flow.
	Digits = 8
	// TimeStep is the RFC 6238 default validity period.
	TimeStep = 30 * time.Second
	// SecretBytes is a 160-bit secret, stronger than the RFC 4226 minimum.
	SecretBytes = 20
	// MaxWindow limits clock-skew acceptance to the RFC-recommended one step.
	MaxWindow = 1
)

var (
	ErrInvalidSecret = errors.New("TOTP secret must contain at least 128 bits")
	ErrInvalidCode   = errors.New("TOTP code must contain exactly eight digits")
	ErrInvalidWindow = errors.New("TOTP window must be between zero and one step")
	ErrReplay        = errors.New("TOTP code was already accepted")
)

// GenerateSecret returns cryptographically random seed material for a new
// authenticator enrollment. Callers must protect the returned bytes like a
// password; they are not recoverable if discarded.
func GenerateSecret() ([]byte, error) {
	secret := make([]byte, SecretBytes)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate TOTP secret: %w", err)
	}
	return secret, nil
}

// Generate returns the RFC 6238 SHA-1 TOTP for now. SHA-1 here is the
// standardized HOTP/TOTP construction; the random secret, TLS channel, and
// protected storage are the security boundaries around it.
func Generate(secret []byte, now time.Time) (string, error) {
	if err := validateSecret(secret); err != nil {
		return "", err
	}
	if now.Unix() < 0 {
		return "", errors.New("TOTP time must not precede the Unix epoch")
	}
	return codeAt(secret, now.Unix()/int64(TimeStep/time.Second)), nil
}

// Verify checks a code in the current timestep and at most one adjacent
// timestep. It does not provide replay protection; use Verifier for account
// logins where a successful code must not be accepted twice.
func Verify(secret []byte, code string, now time.Time, window int) bool {
	if validateSecret(secret) != nil || validateCode(code) != nil || validateWindow(window) != nil || now.Unix() < 0 {
		return false
	}
	return verifyAt(secret, code, now.Unix()/int64(TimeStep/time.Second), window)
}

// Verifier adds per-account replay protection to Verify. A successful code is
// accepted only once and older codes cannot be replayed after a newer one.
type Verifier struct {
	mu           sync.Mutex
	lastAccepted map[string]int64
}

// NewVerifier creates an in-memory replay tracker. Persisting this state is an
// application responsibility when authentication must survive process restarts.
func NewVerifier() *Verifier {
	return &Verifier{lastAccepted: make(map[string]int64)}
}

// Verify checks and consumes a code for account. Empty account identifiers are
// rejected so callers cannot accidentally share one replay bucket.
func (v *Verifier) Verify(account string, secret []byte, code string, now time.Time, window int) error {
	if v == nil {
		return errors.New("TOTP verifier is nil")
	}
	if strings.TrimSpace(account) == "" {
		return errors.New("TOTP account is required")
	}
	if err := validateSecret(secret); err != nil {
		return err
	}
	if err := validateCode(code); err != nil {
		return err
	}
	if err := validateWindow(window); err != nil {
		return err
	}
	if now.Unix() < 0 {
		return errors.New("TOTP time must not precede the Unix epoch")
	}
	counter := now.Unix() / int64(TimeStep/time.Second)
	v.mu.Lock()
	defer v.mu.Unlock()
	if last, ok := v.lastAccepted[account]; ok && counter <= last {
		return ErrReplay
	}
	if !verifyAt(secret, code, counter, window) {
		return errors.New("invalid TOTP code")
	}
	// A code from a future-skew window still advances the replay floor to the
	// timestep it represented, preventing it from being accepted again later.
	for offset := -window; offset <= window; offset++ {
		candidate := counter + int64(offset)
		if hmac.Equal([]byte(codeAt(secret, candidate)), []byte(code)) {
			v.lastAccepted[account] = candidate
			return nil
		}
	}
	return errors.New("invalid TOTP code")
}

// ProvisioningURI creates a standard otpauth URI for QR enrollment. The URI
// contains the secret by design; never log it or send it over plaintext HTTP.
func ProvisioningURI(issuer, account string, secret []byte) (string, error) {
	if err := validateSecret(secret); err != nil {
		return "", err
	}
	issuer = strings.TrimSpace(issuer)
	account = strings.TrimSpace(account)
	if issuer == "" || account == "" {
		return "", errors.New("TOTP issuer and account are required")
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
	values := url.Values{}
	values.Set("secret", encoded)
	values.Set("issuer", issuer)
	values.Set("algorithm", "SHA1")
	values.Set("digits", strconv.Itoa(Digits))
	values.Set("period", strconv.Itoa(int(TimeStep/time.Second)))
	return "otpauth://totp/" + url.PathEscape(issuer+":"+account) + "?" + values.Encode(), nil
}

// EncryptSecret protects an enrolled secret with an application-managed
// 256-bit key. The returned value is nonce || ciphertext; AES-GCM authenticates
// both, and a fresh random nonce is generated for every call.
func EncryptSecret(secret, key []byte) ([]byte, error) {
	if err := validateSecret(secret); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("TOTP encryption key must be exactly 32 bytes")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate TOTP nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, secret, nil), nil
}

// DecryptSecret authenticates and decrypts a value returned by EncryptSecret.
func DecryptSecret(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("TOTP encryption key must be exactly 32 bytes")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("encrypted TOTP secret is truncated")
	}
	secret, err := gcm.Open(nil, ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():], nil)
	if err != nil {
		return nil, errors.New("encrypted TOTP secret failed authentication")
	}
	if err := validateSecret(secret); err != nil {
		return nil, err
	}
	return secret, nil
}

func verifyAt(secret []byte, code string, counter int64, window int) bool {
	for offset := -window; offset <= window; offset++ {
		candidate := counter + int64(offset)
		if candidate >= 0 && subtle.ConstantTimeCompare([]byte(codeAt(secret, candidate)), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

func codeAt(secret []byte, counter int64) string {
	var message [8]byte
	binary.BigEndian.PutUint64(message[:], uint64(counter))
	digest := hmac.New(sha1.New, secret)
	_, _ = digest.Write(message[:])
	sum := digest.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	modulus := uint32(100000000)
	return fmt.Sprintf("%08d", value%modulus)
}

func validateSecret(secret []byte) error {
	if len(secret) < 16 {
		return ErrInvalidSecret
	}
	return nil
}

func validateCode(code string) error {
	if len(code) != Digits {
		return ErrInvalidCode
	}
	for _, character := range code {
		if character < '0' || character > '9' {
			return ErrInvalidCode
		}
	}
	return nil
}

func validateWindow(window int) error {
	if window < 0 || window > MaxWindow {
		return ErrInvalidWindow
	}
	return nil
}
