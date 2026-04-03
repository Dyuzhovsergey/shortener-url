// Package certutil for generate
package certutil

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/url"
	"strings"
	"time"
)

// GenerateSelfSignedCertificate создаёт self-signed TLS-сертификат в памяти.
// Его достаточно для локального HTTPS в учебном проекте.
func GenerateSelfSignedCertificate(baseURL, runAddr string) (tls.Certificate, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return tls.Certificate{}, err
	}

	dnsNames, ipAddresses := collectHosts(baseURL, runAddr)

	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"shortener-url"},
			Country:      []string{"RU"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		BasicConstraintsValid: true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		DNSNames:              dnsNames,
		IPAddresses:           ipAddresses,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, cert, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, err
	}

	var certPEM bytes.Buffer
	if err := pem.Encode(&certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}); err != nil {
		return tls.Certificate{}, err
	}

	var keyPEM bytes.Buffer
	if err := pem.Encode(&keyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}); err != nil {
		return tls.Certificate{}, err
	}

	return tls.X509KeyPair(certPEM.Bytes(), keyPEM.Bytes())
}

// collectHosts собирает DNS-имена и IP-адреса, которые нужно положить в сертификат.
func collectHosts(baseURL, runAddr string) ([]string, []net.IP) {
	dnsSet := map[string]struct{}{
		"localhost": {},
	}
	ipSet := map[string]net.IP{
		"127.0.0.1": net.IPv4(127, 0, 0, 1),
		"::1":       net.IPv6loopback,
	}

	addHost := func(host string) {
		host = strings.TrimSpace(host)
		if host == "" {
			return
		}

		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}

		host = strings.Trim(host, "[]")
		if host == "" {
			return
		}

		if ip := net.ParseIP(host); ip != nil {
			ipSet[ip.String()] = ip
			return
		}

		dnsSet[host] = struct{}{}
	}

	if u, err := url.Parse(baseURL); err == nil {
		addHost(u.Host)
	}

	addHost(runAddr)

	dnsNames := make([]string, 0, len(dnsSet))
	for host := range dnsSet {
		dnsNames = append(dnsNames, host)
	}

	ipAddresses := make([]net.IP, 0, len(ipSet))
	for _, ip := range ipSet {
		ipAddresses = append(ipAddresses, ip)
	}

	return dnsNames, ipAddresses
}
