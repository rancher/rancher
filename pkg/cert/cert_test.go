package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"math/big"
	"testing"
)

func TestMatchAndKeySizeRSA(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	algo, size, ok := matchAndKeySize(&priv.PublicKey, priv)
	if !ok {
		t.Fatal("expected RSA public key to match its private key")
	}
	if algo != "RSA" {
		t.Errorf("expected algorithm RSA, got %q", algo)
	}
	if size != len(priv.N.Bytes()) {
		t.Errorf("expected key size %d, got %d", len(priv.N.Bytes()), size)
	}

	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate second RSA key: %v", err)
	}
	if _, _, ok := matchAndKeySize(&other.PublicKey, priv); ok {
		t.Error("expected mismatched RSA public key not to match the private key")
	}
}

func TestMatchAndKeySizeECDSA(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ECDSA key: %v", err)
	}

	algo, size, ok := matchAndKeySize(&priv.PublicKey, priv)
	if !ok {
		t.Fatal("expected ECDSA public key to match its private key")
	}
	if algo != "ECC" {
		t.Errorf("expected algorithm ECC, got %q", algo)
	}
	if size != 256 {
		t.Errorf("expected key size 256, got %d", size)
	}

	// A public key that shares the X coordinate but has a different Y
	// coordinate does not belong to this private key and must not be
	// reported as a match.
	wrongY := &ecdsa.PublicKey{
		Curve: priv.Curve,
		X:     new(big.Int).Set(priv.X),
		Y:     new(big.Int).Add(priv.Y, big.NewInt(1)),
	}
	if _, _, ok := matchAndKeySize(wrongY, priv); ok {
		t.Error("expected ECDSA public key with mismatched Y coordinate not to match the private key")
	}

	// A completely different key must not match either.
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate second ECDSA key: %v", err)
	}
	if _, _, ok := matchAndKeySize(&other.PublicKey, priv); ok {
		t.Error("expected mismatched ECDSA public key not to match the private key")
	}
}
