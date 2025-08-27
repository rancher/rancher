package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

// ensureSAKeypairForCluster loads or creates an RSA keypair used to mint service-account-like JWTs
func ensureSAKeypairForCluster(clusterName string) (*rsa.PrivateKey, *rsa.PublicKey, error) {
    pkiDir := filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", clusterName, "pki")
    if err := os.MkdirAll(pkiDir, 0o755); err != nil {
        return nil, nil, fmt.Errorf("mkdir pki: %w", err)
    }
    privPath := filepath.Join(pkiDir, "sa.key")
    pubPath := filepath.Join(pkiDir, "sa.pub")

    // Try load private key
    if b, err := os.ReadFile(privPath); err == nil {
        priv, perr := parseRSAPrivateKeyFromPEM(b)
        if perr != nil {
            return nil, nil, fmt.Errorf("parse sa.key: %w", perr)
        }
        return priv, &priv.PublicKey, nil
    }

    // Generate new keypair
    priv, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        return nil, nil, fmt.Errorf("gen rsa: %w", err)
    }
    if err := writeRSAPrivateKeyPEM(privPath, priv); err != nil {
        return nil, nil, err
    }
    if err := writeRSAPublicKeyPEM(pubPath, &priv.PublicKey); err != nil {
        return nil, nil, err
    }
    return priv, &priv.PublicKey, nil
}

func parseRSAPrivateKeyFromPEM(pemBytes []byte) (*rsa.PrivateKey, error) {
    block, _ := pem.Decode(pemBytes)
    if block == nil || block.Type != "RSA PRIVATE KEY" {
        return nil, errors.New("invalid RSA PRIVATE KEY pem")
    }
    return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func writeRSAPrivateKeyPEM(path string, k *rsa.PrivateKey) error {
    f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
    if err != nil {
        return fmt.Errorf("open priv: %w", err)
    }
    defer f.Close()
    if err := pem.Encode(f, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}); err != nil {
        return fmt.Errorf("encode priv: %w", err)
    }
    return nil
}

func writeRSAPublicKeyPEM(path string, k *rsa.PublicKey) error {
    f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
    if err != nil {
        return fmt.Errorf("open pub: %w", err)
    }
    defer f.Close()
    der, err := x509.MarshalPKIXPublicKey(k)
    if err != nil {
        return fmt.Errorf("marshal pub: %w", err)
    }
    if err := pem.Encode(f, &pem.Block{Type: "PUBLIC KEY", Bytes: der}); err != nil {
        return fmt.Errorf("encode pub: %w", err)
    }
    return nil
}

// mintServiceAccountJWTWithKey mints a RS256 JWT with SA-style claims
func mintServiceAccountJWTWithKey(priv *rsa.PrivateKey, namespace, name string, audience []string, ttl time.Duration) (string, error) {
    now := time.Now()
    claims := jwt.RegisteredClaims{
        Issuer:    "kubernetes/serviceaccount",
        Subject:   fmt.Sprintf("system:serviceaccount:%s:%s", namespace, name),
        IssuedAt:  jwt.NewNumericDate(now),
        NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
        ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
    }
    if len(audience) > 0 {
        claims.Audience = jwt.ClaimStrings(audience)
    }
    token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
    return token.SignedString(priv)
}

// verifyServiceAccountJWT verifies the RS256 JWT and basic SA-style claims
func verifyServiceAccountJWT(tokenString string, pub *rsa.PublicKey, namespace, name string) error {
    var claims jwt.RegisteredClaims
    tok, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"]) 
        }
        return pub, nil
    })
    if err != nil {
        return err
    }
    if !tok.Valid {
        return errors.New("invalid token")
    }
    // Basic subject check
    wantSub := fmt.Sprintf("system:serviceaccount:%s:%s", namespace, name)
    if claims.Subject != wantSub {
        return fmt.Errorf("unexpected subject: %s", claims.Subject)
    }
    return nil
}
