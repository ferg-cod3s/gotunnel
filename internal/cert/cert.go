package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
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

	// Check if cert already exists for the provided domain
	if _, err := os.Stat(certPath); err == nil {
		if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
			return &cert, nil
		}
	}

	// Generate new cert if it does not
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: domain,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(1, 0, 0),

		//	Key Usage info
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain},
	}

	// create certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// save cert and key

	// make cert directory
	if err := os.MkdirAll(cm.certsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create certs directory: %w", err)
	}

	//make cert file
	certOut, err := os.Create(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create cert file: %w", err)
	}

	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	// create key file
	keyOut, err := os.Create(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create key file: %w", err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	keyOut.Close()

	//finish loading the cert
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load generated cert: %w", err)
	}

	return &cert, nil
}
