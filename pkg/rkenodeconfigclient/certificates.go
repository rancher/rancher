package rkenodeconfigclient

import (
	"crypto/tls"
	"crypto/x509"
	"time"

	"github.com/sirupsen/logrus"
)

const kubeletCertValidityLimit = time.Hour * 72

func kubeletCertificateNeedsRegeneration(ipAddress, currentHostname string, cert tls.Certificate, currentTime time.Time) (bool, error) {
	if len(cert.Certificate) == 0 {
		return true, nil
	}

	parsedCert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return false, err
	}

	if !certificateIncludesHostname(currentHostname, parsedCert) {
		logrus.Tracef("certificate does not include current hostname, requesting new certificate")
		return true, nil
	}

	if certificateIsExpiring(parsedCert, currentTime) {
		logrus.Tracef("certificate is expiring soon, requesting new certificate")
		return true, nil
	}

	if !certificateIncludesCurrentIP(ipAddress, parsedCert) {
		logrus.Tracef("certificate does not include current IP address, requesting new certificate")
		return true, nil
	}

	return false, nil
}

// certificateIsExpiring checks if the passed certificate will expire within
// the kubeletCertValidityLimit
func certificateIsExpiring(cert *x509.Certificate, currentTime time.Time) bool {
	return cert.NotAfter.Sub(currentTime) < kubeletCertValidityLimit
}

// certificateIncludesHostname checks that the passed certificate includes
// the provided hostname in its SAN list
func certificateIncludesHostname(hostname string, cert *x509.Certificate) bool {
	for _, name := range cert.DNSNames {
		if name == hostname {
			return true
		}
	}
	return false
}

// certificateIncludesCurrentIP checks that the passed certificate includes the provided IP address
func certificateIncludesCurrentIP(ipAddress string, cert *x509.Certificate) bool {
	for _, ip := range cert.IPAddresses {
		if ipAddress == ip.String() {
			return true
		}
	}
	return false
}
