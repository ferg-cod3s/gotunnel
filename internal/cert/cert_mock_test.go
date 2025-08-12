package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateTestCertificate creates a self-signed certificate for testing
func generateTestCertificate(domain string) (certPEM, keyPEM []byte, err error) {
	// Generate RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test Organization"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		DNSNames:     []string{domain},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  nil,
	}

	// Generate certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}

	// Encode certificate to PEM
	certPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key to PEM
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyDER,
	})

	return certPEM, keyPEM, nil
}

func TestEnsureCertWithMockCert(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cert-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	domain := "test.local"
	certFile := filepath.Join(tempDir, domain+".pem")
	keyFile := filepath.Join(tempDir, domain+"-key.pem")

	// Generate test certificate
	certPEM, keyPEM, err := generateTestCertificate(domain)
	require.NoError(t, err)

	// Write certificate files
	err = os.WriteFile(certFile, certPEM, 0644)
	require.NoError(t, err)
	err = os.WriteFile(keyFile, keyPEM, 0600)
	require.NoError(t, err)

	// Test loading the certificate
	cm := New(tempDir)
	cert, err := cm.EnsureCert(domain)
	require.NoError(t, err)
	require.NotNil(t, cert)

	// Verify certificate properties
	assert.Len(t, cert.Certificate, 1) // Should have one certificate in chain
	
	// Parse and verify the certificate
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	assert.Contains(t, x509Cert.DNSNames, domain)
	assert.True(t, time.Now().Before(x509Cert.NotAfter))
}

func TestCertificateReuse(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cert-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	domain := "reuse-test.local"
	
	// Generate and save test certificate
	certPEM, keyPEM, err := generateTestCertificate(domain)
	require.NoError(t, err)

	certFile := filepath.Join(tempDir, domain+".pem")
	keyFile := filepath.Join(tempDir, domain+"-key.pem")
	
	err = os.WriteFile(certFile, certPEM, 0644)
	require.NoError(t, err)
	err = os.WriteFile(keyFile, keyPEM, 0600)
	require.NoError(t, err)

	cm := New(tempDir)

	// Load certificate first time
	cert1, err := cm.EnsureCert(domain)
	require.NoError(t, err)
	require.NotNil(t, cert1)

	// Load certificate second time (should reuse existing)
	cert2, err := cm.EnsureCert(domain)
	require.NoError(t, err)
	require.NotNil(t, cert2)

	// Should be the same certificate
	assert.Equal(t, cert1.Certificate[0], cert2.Certificate[0])
}

func TestCertificateDirectoryCreation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cert-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Use a subdirectory that doesn't exist yet
	certsDir := filepath.Join(tempDir, "certs", "subdirectory")
	domain := "directory-test.local"

	// Generate and save test certificate in non-existent directory
	certPEM, keyPEM, err := generateTestCertificate(domain)
	require.NoError(t, err)

	cm := New(certsDir)

	// Create directory first (EnsureCert should do this)
	err = os.MkdirAll(certsDir, 0755)
	require.NoError(t, err)

	certFile := filepath.Join(certsDir, domain+".pem")
	keyFile := filepath.Join(certsDir, domain+"-key.pem")
	
	err = os.WriteFile(certFile, certPEM, 0644)
	require.NoError(t, err)
	err = os.WriteFile(keyFile, keyPEM, 0600)
	require.NoError(t, err)

	// Should successfully load the certificate
	cert, err := cm.EnsureCert(domain)
	require.NoError(t, err)
	require.NotNil(t, cert)
}

func TestInvalidCertificateHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cert-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	domain := "invalid-test.local"
	certFile := filepath.Join(tempDir, domain+".pem")
	keyFile := filepath.Join(tempDir, domain+"-key.pem")

	// Create invalid certificate files
	err = os.WriteFile(certFile, []byte("invalid cert data"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(keyFile, []byte("invalid key data"), 0600)
	require.NoError(t, err)

	cm := New(tempDir)

	// Should fail to load invalid certificate
	cert, err := cm.EnsureCert(domain)
	assert.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "failed to load existing certificate")
}