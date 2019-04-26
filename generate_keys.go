// MARK: Key generation
// see https://golang.org/src/crypto/tls/generate_cert.go
// this is essentially the above with almost everything removed
// except the minimum code to generate a self-signed ed25519 key pair

package main

import (
  "crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"
)

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func pemBlockForKey(priv interface{}) *pem.Block {
	switch k := priv.(type) {
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal ECDSA private key: %v", err)
			os.Exit(2)
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return nil
	}
}

func generate_cert(path string, name string) error {

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	notBefore := time.Now()

	notAfter := notBefore.Add(10 * 365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("failed to generate serial number: %s", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"P-Drop Application"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// hosts := strings.Split(*host, ",")
	// for _, h := range hosts {
	// 	if ip := net.ParseIP(h); ip != nil {
	// 		template.IPAddresses = append(template.IPAddresses, ip)
	// 	} else {
	// 		template.DNSNames = append(template.DNSNames, h)
	// 	}
	// }

	host, err := os.Hostname()
	if err != nil {
		return err
	}

	template.DNSNames = append(template.DNSNames, host)

	// if *isCA {
	// 	template.IsCA = true
	// 	template.KeyUsage |= x509.KeyUsageCertSign
	// }

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %s", err)
	}

	certPath := path + "/" + name + ".crt"
	keyPath := path + "/" + name + ".key"

	certOut, err := os.Create(certPath)
	if err != nil {
		log.Fatalf("failed to open %s for writing: %s", certPath, err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		log.Fatalf("failed to write data to %s: %s", certPath, err)
	}
	if err := certOut.Close(); err != nil {
		log.Fatalf("error closing %s: %s", certPath, err)
	}
	log.Printf("wrote %s\n", certPath)

	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Print("failed to open %s for writing: %s", keyPath, err)
		return err
	}
	if err := pem.Encode(keyOut, pemBlockForKey(priv)); err != nil {
		log.Fatalf("failed to write data to %s: %s", keyPath, err)
	}
	if err := keyOut.Close(); err != nil {
		log.Fatalf("error closing key.pem: %s", err)
	}
	log.Printf("wrote %s\n", keyPath)

	return nil
}
