package main

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHostTLSLifecycleUsesCertificateAndKey(t *testing.T) {
	clearChessEnv(t)
	certificate, key := writeTestTLSMaterial(t)
	writer := &notifyingWriter{ready: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runHost(ctx, []string{"--addr", "127.0.0.1:0", "--cert", certificate, "--key", key}, writer)
	}()
	select {
	case <-writer.ready:
		cancel()
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("TLS host did not start listening")
	}
	if err := <-done; err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("TLS host shutdown error = %v", err)
	}
}

func writeTestTLSMaterial(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := cryptorand.Int(cryptorand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "chess-go test"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(cryptorand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	certificatePath := filepath.Join(directory, "server.crt")
	keyPath := filepath.Join(directory, "server.key")
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(certificatePath, certificatePEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	return certificatePath, keyPath
}
