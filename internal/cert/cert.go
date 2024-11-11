package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type CertManager struct {
	certsDir string
}

func New(certsDir string) *CertManager {
	return &CertManager{
		certsDir: certsDir,
	}
}

func (cm *CertManager) EnsureCert(domain string) (*tls.Certificate, error) {
	certPath := filepath.Join(cm.certsDir, domain+".crt")
	keyPath := filepath.Join(cm.certsDir, domain+".key")

	log.Printf("Ensuring certificate for domain: %s", domain)
	if _, err := os.Stat(certPath); err == nil {
		log.Printf("Certificate already exists for domain: %s", domain)
		if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
			return &cert, nil
		}
	}

	log.Printf("Generating new certificate for domain: %s", domain)
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: domain,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	if err := os.MkdirAll(cm.certsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create certs directory: %w", err)
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create cert file: %w", err)
	}
	defer certOut.Close()

	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	keyOut, err := os.Create(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyOut.Close()

	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load generated cert: %w", err)
	}

	log.Printf("Successfully ensured certificate for domain: %s", domain)
	return &cert, nil
}
