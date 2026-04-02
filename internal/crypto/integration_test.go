package crypto

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

// TestDecryptAEAD_WrongAAD verifies that decryption fails when AAD does not match.
func TestDecryptAEAD_WrongAAD(t *testing.T) {
	var key [32]byte
	plaintext := []byte("secret data")
	aad := []byte("correct-aad")
	wrongAAD := []byte("wrong-aad")

	ct, err := EncryptAEAD(key, 0, plaintext, aad)
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecryptAEAD(key, 0, ct, wrongAAD)
	if err == nil {
		t.Fatal("decryption with wrong AAD should fail")
	}
}

// TestEncryptAEAD_EmptyPlaintext verifies encryption/decryption of empty plaintext.
func TestEncryptAEAD_EmptyPlaintext(t *testing.T) {
	var key [32]byte
	ct, err := EncryptAEAD(key, 0, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ct) == 0 {
		t.Fatal("ciphertext of empty plaintext should still contain auth tag")
	}
	pt, err := DecryptAEAD(key, 0, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pt) != 0 {
		t.Fatalf("expected empty plaintext, got %q", pt)
	}
}

// TestEncryptAEAD_LargePlaintext verifies encryption/decryption of larger data.
func TestEncryptAEAD_LargePlaintext(t *testing.T) {
	var key [32]byte
	plaintext := make([]byte, 4096)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	ct, err := EncryptAEAD(key, 42, plaintext, nil)
	if err != nil {
		t.Fatal(err)
	}
	pt, err := DecryptAEAD(key, 42, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pt, plaintext) {
		t.Fatal("large plaintext round-trip failed")
	}
}

// TestMakeNonce12_Encoding verifies the nonce layout (4 zero bytes + 8-byte LE counter).
func TestMakeNonce12_Encoding(t *testing.T) {
	n := makeNonce12(0)
	for i := 0; i < 4; i++ {
		if n[i] != 0 {
			t.Fatalf("first 4 bytes should be zero, got n[%d]=%d", i, n[i])
		}
	}
	got := binary.LittleEndian.Uint64(n[4:])
	if got != 0 {
		t.Fatalf("counter at nonce[4:] should be 0, got %d", got)
	}

	n = makeNonce12(1)
	got = binary.LittleEndian.Uint64(n[4:])
	if got != 1 {
		t.Fatalf("counter at nonce[4:] should be 1, got %d", got)
	}

	n = makeNonce12(0xDEADBEEF)
	got = binary.LittleEndian.Uint64(n[4:])
	if got != 0xDEADBEEF {
		t.Fatalf("counter at nonce[4:] should be 0xDEADBEEF, got 0x%X", got)
	}
}

// TestEncryptDecrypt_SequentialCounters verifies encryption under sequential counters.
func TestEncryptDecrypt_SequentialCounters(t *testing.T) {
	var key [32]byte
	plaintext := []byte("same message")

	ct0, _ := EncryptAEAD(key, 0, plaintext, nil)
	ct1, _ := EncryptAEAD(key, 1, plaintext, nil)
	ct2, _ := EncryptAEAD(key, 2, plaintext, nil)

	// All ciphertexts should be different due to different nonces.
	if bytes.Equal(ct0, ct1) {
		t.Fatal("counter 0 and 1 produced same ciphertext")
	}
	if bytes.Equal(ct1, ct2) {
		t.Fatal("counter 1 and 2 produced same ciphertext")
	}

	// Each should decrypt correctly under its own counter.
	for i, ct := range [][]byte{ct0, ct1, ct2} {
		pt, err := DecryptAEAD(key, uint64(i), ct, nil)
		if err != nil {
			t.Fatalf("decrypt counter %d: %v", i, err)
		}
		if !bytes.Equal(pt, plaintext) {
			t.Fatalf("counter %d: plaintext mismatch", i)
		}
	}
}

// TestDecryptAEAD_TruncatedCiphertext verifies decryption fails with too-short input.
func TestDecryptAEAD_TruncatedCiphertext(t *testing.T) {
	var key [32]byte
	// Auth tag alone is 16 bytes; anything less should fail.
	_, err := DecryptAEAD(key, 0, []byte{0x01, 0x02, 0x03}, nil)
	if err == nil {
		t.Fatal("decryption of truncated ciphertext should fail")
	}
}

// TestLoadKeyPair_MissingDir verifies LoadKeyPair returns error when directory does not exist.
func TestLoadKeyPair_MissingDir(t *testing.T) {
	_, err := LoadKeyPair("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

// TestLoadKeyPair_EmptyPrivateKey verifies LoadKeyPair returns error for empty key file.
func TestLoadKeyPair_EmptyPrivateKey(t *testing.T) {
	dir := t.TempDir()
	privPath := dir + "/private.key"
	if err := os.WriteFile(privPath, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadKeyPair(dir)
	if err == nil {
		t.Fatal("expected error for empty private key file")
	}
}

// TestSaveLoadKeyPair_Overwrite verifies that saving over existing keys works.
func TestSaveLoadKeyPair_Overwrite(t *testing.T) {
	dir := t.TempDir()
	kp1, _ := GenerateKeyPair()
	SaveKeyPair(kp1, dir)

	kp2, _ := GenerateKeyPair()
	if kp1.Public == kp2.Public {
		t.Fatal("two generated key pairs should differ")
	}
	SaveKeyPair(kp2, dir)

	loaded, err := LoadKeyPair(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Public != kp2.Public {
		t.Fatal("loaded key should be the second (overwritten) key pair")
	}
}

// TestHMAC_DifferentData verifies HMAC produces different outputs for different data.
func TestHMAC_DifferentData(t *testing.T) {
	key := []byte("key")
	a := HMAC(key, []byte("data1"))
	b := HMAC(key, []byte("data2"))
	if a == b {
		t.Fatal("HMAC should differ for different data")
	}
}

// TestHMAC_EmptyKey verifies HMAC works with an empty key.
func TestHMAC_EmptyKey(t *testing.T) {
	result := HMAC([]byte{}, []byte("data"))
	var zero [32]byte
	if result == zero {
		t.Fatal("HMAC with empty key should produce non-zero output")
	}
}

// TestMixHash_DifferentInputs verifies MixHash produces different outputs for different inputs.
func TestMixHash_DifferentInputs(t *testing.T) {
	var h [32]byte
	h[0] = 0xAA
	a := MixHash(h, []byte("data1"))
	b := MixHash(h, []byte("data2"))
	if a == b {
		t.Fatal("MixHash should differ for different data inputs")
	}
}

// TestMixHash_SameHashInput verifies MixHash with same hash but different data.
func TestMixHash_SameHashInput(t *testing.T) {
	var h1, h2 [32]byte
	h1[0] = 0xAA
	h2[0] = 0xBB
	data := []byte("same-data")
	a := MixHash(h1, data)
	b := MixHash(h2, data)
	if a == b {
		t.Fatal("MixHash should differ for different hash states")
	}
}

// TestSplit_DifferentInputs verifies Split produces different outputs for different chain keys.
func TestSplit_DifferentInputs(t *testing.T) {
	var ck1, ck2 [32]byte
	ck1[0] = 0xAA
	ck2[0] = 0xBB
	k1a, k1b := Split(ck1)
	k2a, k2b := Split(ck2)
	if k1a == k2a {
		t.Fatal("Split should produce different k1 for different chain keys")
	}
	if k1b == k2b {
		t.Fatal("Split should produce different k2 for different chain keys")
	}
}

// TestNoiseHandshake_RemoteStaticKeyBeforeComplete verifies RemoteStaticKey on initiator before handshake.
func TestNoiseHandshake_RemoteStaticKeyBeforeComplete(t *testing.T) {
	ikp, _ := GenerateKeyPair()
	rkp, _ := GenerateKeyPair()
	hs, _ := InitiatorHandshake(ikp, rkp.Public)
	// Initiator should have the remote static key set from the start.
	got := hs.RemoteStaticKey()
	if got != rkp.Public {
		t.Fatal("initiator should have remote static key from construction")
	}
}

// TestNoiseHandshake_ResponderRemoteStaticBeforeRead verifies responder has zero remote static before ReadMessage1.
func TestNoiseHandshake_ResponderRemoteStaticBeforeRead(t *testing.T) {
	rkp, _ := GenerateKeyPair()
	hs, _ := ResponderHandshake(rkp)
	got := hs.RemoteStaticKey()
	if !got.IsZero() {
		t.Fatal("responder should have zero remote static before ReadMessage1")
	}
}

// TestReplayWindow_ConcurrentAccess verifies thread safety under concurrent Check/Advance.
func TestReplayWindow_ConcurrentAccess(t *testing.T) {
	var w ReplayWindow
	done := make(chan struct{})

	// Writer goroutine: advances counters 0..999.
	go func() {
		defer close(done)
		for i := uint64(0); i < 1000; i++ {
			w.Advance(i)
		}
	}()

	// Reader goroutine: checks random counters.
	go func() {
		for i := uint64(0); i < 1000; i++ {
			w.Check(i) // result doesn't matter; must not race
		}
	}()

	<-done
}

// TestReplayWindow_Advance_SameCounterTwice verifies Advance handles duplicate calls.
func TestReplayWindow_Advance_SameCounterTwice(t *testing.T) {
	var w ReplayWindow
	w.Advance(42)
	w.Advance(42) // should be no-op, not panic
	if w.Check(42) {
		t.Fatal("counter 42 should still be rejected after double advance")
	}
}

// TestReplayWindow_CheckDoesNotModify verifies that Check does not change window state.
func TestReplayWindow_CheckDoesNotModify(t *testing.T) {
	var w ReplayWindow
	// Check counter 0 before advancing.
	if !w.Check(0) {
		t.Fatal("counter 0 should be accepted before Advance")
	}
	// Check again — should still be accepted (Check is read-only).
	if !w.Check(0) {
		t.Fatal("counter 0 should still be accepted (Check should not modify state)")
	}
	w.Advance(0)
	if w.Check(0) {
		t.Fatal("counter 0 should be rejected after Advance")
	}
}

// TestReplayWindow_SlideByExactWindow verifies window slides correctly at WindowSize boundary.
func TestReplayWindow_SlideByExactWindow(t *testing.T) {
	var w ReplayWindow
	// Advance to fill the window exactly.
	w.Advance(WindowSize - 1)
	// Counter 0 should now be accepted (it's at the edge).
	if !w.Check(0) {
		t.Fatal("counter 0 should still be in window")
	}
	// Advance one more to push counter 0 out.
	w.Advance(WindowSize)
	if w.Check(0) {
		t.Fatal("counter 0 should be rejected after window slides past it")
	}
}

// TestGenerateKeyPair_Uniqueness verifies multiple generated key pairs are all distinct.
func TestGenerateKeyPair_Uniqueness(t *testing.T) {
	keys := make(map[string]bool)
	for i := 0; i < 10; i++ {
		kp, err := GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}
		s := kp.Public.String()
		if keys[s] {
			t.Fatalf("duplicate public key generated at iteration %d", i)
		}
		keys[s] = true
	}
}
