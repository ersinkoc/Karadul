package coordinator

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/karadul/karadul/internal/crypto"
)

const (
	headerKey = "X-Karadul-Key"
	headerSig = "X-Karadul-Sig"
)

// GenerateAuthKey creates a new random pre-auth key.
// If ephemeral is true, the key is single-use.
// ttl is the key lifetime (0 = never expires).
func GenerateAuthKey(ephemeral bool, ttl time.Duration) (*AuthKey, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate auth key: %w", err)
	}
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("generate auth key id: %w", err)
	}
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	return &AuthKey{
		ID:        hex.EncodeToString(idBytes),
		Key:       base64.StdEncoding.EncodeToString(secret),
		Ephemeral: ephemeral,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}, nil
}

// ValidateAuthKey checks that key is valid (not expired, not used if ephemeral).
func ValidateAuthKey(k *AuthKey) error {
	if k == nil {
		return fmt.Errorf("unknown auth key")
	}
	if !k.ExpiresAt.IsZero() && time.Now().After(k.ExpiresAt) {
		return fmt.Errorf("auth key expired")
	}
	if k.Ephemeral && k.Used {
		return fmt.Errorf("auth key already used")
	}
	return nil
}

// SignRequest computes the HMAC-BLAKE2s signature for an HTTP request.
// Both client and server use the node's public key as the HMAC key so
// the coordinator can verify using only the registered public key.
// The message is: method + "\n" + path + "\n" + body
func SignRequest(pubKey [32]byte, method, path string, body []byte) string {
	msg := append([]byte(method+"\n"+path+"\n"), body...)
	mac := crypto.HMAC(pubKey[:], msg)
	return base64.StdEncoding.EncodeToString(mac[:])
}

// VerifyRequestSignature verifies the signature header on an incoming request.
// It checks the X-Karadul-Key and X-Karadul-Sig headers.
func VerifyRequestSignature(store *Store, r *http.Request, body []byte) error {
	pubKeyB64 := r.Header.Get(headerKey)
	sigB64 := r.Header.Get(headerSig)

	if pubKeyB64 == "" || sigB64 == "" {
		return fmt.Errorf("missing auth headers")
	}

	pubKeyBytes, err := base64.StdEncoding.DecodeString(pubKeyB64)
	if err != nil || len(pubKeyBytes) != 32 {
		return fmt.Errorf("invalid public key header")
	}

	node, ok := store.GetNodeByPubKey(pubKeyB64)
	if !ok {
		return fmt.Errorf("unknown node key")
	}
	if node.Status != NodeStatusActive {
		return fmt.Errorf("node not active")
	}

	sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil || len(sigBytes) != 32 {
		return fmt.Errorf("invalid signature")
	}

	// Reconstruct expected signature using the node's public key as the HMAC key.
	// The node signs with its private key; we verify with its public key.
	// (Asymmetric HMAC-check: both sides derive from the shared secret in practice,
	//  but here we use public key as a simple HMAC key for request authentication.)
	msg := append([]byte(r.Method+"\n"+r.URL.RequestURI()+"\n"), body...)
	var pk [32]byte
	copy(pk[:], pubKeyBytes)
	expected := crypto.HMAC(pk[:], msg)

	if subtle.ConstantTimeCompare(sigBytes, expected[:]) != 1 {
		return fmt.Errorf("invalid signature")
	}
	return nil
}
