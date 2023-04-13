package rkenodeconfigclient

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
	"testing"
	"time"

	"github.com/rancher/rke/types"
	"github.com/stretchr/testify/assert"
)

func TestFindCommandValue(t *testing.T) {
	commands := []string{
		"--hostname=test.com",
		"--tls-cert-file=a-test-value",
		"---abc=123",
	}

	type testcase struct {
		testName string
		flag     string
		value    string
	}

	tests := []testcase{
		{
			testName: "successfully find flag in command arguments",
			value:    "a-test-value",
			flag:     "--tls-cert-file",
		},
		{
			testName: "unsuccessfully find flag in command arguments",
			value:    "",
			flag:     "--not-a-flag",
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			v := findCommandValue(tc.flag, commands)
			if v != tc.value {
				t.Logf("wanted %s, got %s", tc.value, v)
				t.Fail()
			}
		})
	}
}

func TestGetKubeletCertificateFilesFromProcess(t *testing.T) {

	type testcase struct {
		testName            string
		processes           map[string]types.Process
		privateKeyFileValue string
		tlsCertFileValue    string
		want                bool
	}

	testCases := []testcase{
		{
			testName: "successfully find kubelet arg",
			processes: map[string]types.Process{
				"kubelet": {
					// rancher actually passes all args in the 'Command' field, since
					// we use the entrypoint.sh file
					Command: []string{
						"--tls-private-key-file=/etc/kubernetes/ssl/a-private-key.pem",
						"--tls-cert-file=/etc/kubernetes/ssl/a-private.pem",
					},
				},
				"not-kubelet": {},
			},
			privateKeyFileValue: "/etc/kubernetes/ssl/a-private-key.pem",
			tlsCertFileValue:    "/etc/kubernetes/ssl/a-private.pem",
			want:                true,
		},
		{
			testName: "unsuccessfully find kubelet arg",
			processes: map[string]types.Process{
				"kubelet": {
					// rancher actually passes all args in the 'Command' field, since
					// we use the entrypoint.sh file
					Command: []string{},
				},
				"not-kubelet": {},
			},
			privateKeyFileValue: "a-private-key.pem",
			tlsCertFileValue:    "a-private.pem",
			want:                false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			privateKeyFileName, tlsCertFileName := getKubeletCertificateFilesFromProcess(tc.processes)
			if (privateKeyFileName != tc.privateKeyFileValue || tlsCertFileName != tc.tlsCertFileValue) && tc.want {
				t.Logf("wanted %s %s, got %s %s ", tc.tlsCertFileValue, tc.privateKeyFileValue, tlsCertFileName, privateKeyFileName)
				t.Fail()
			}
		})
	}
}

func TestKubeletCertificateNeedsRegeneration(t *testing.T) {

	correctIPAddress := "167.71.188.113"
	correctHostName := "cert-test1"
	testCertExpiryTime := time.Now().AddDate(10, 0, 3)

	type testcase struct {
		testName      string
		testIPAddress string
		testHostName  string
		testTime      time.Time
		want          bool
	}

	testCases := []testcase{
		{
			testName:      "All Cert Properties Are Valid",
			testIPAddress: correctIPAddress,
			testHostName:  correctHostName,
			testTime:      time.Now(),
			want:          false,
		},
		{
			testName:      "Cert Contains Invalid IP Address",
			testIPAddress: "192.168.1.1",
			testHostName:  correctHostName,
			testTime:      time.Now(),
			want:          true,
		},
		{
			testName:      "Cert Contains Invalid Hostname",
			testIPAddress: correctIPAddress,
			testHostName:  "different-hostname",
			testTime:      time.Now(),
			want:          true,
		},
		{
			testName:      "Cert Will Expire In Less Than Three Days",
			testIPAddress: correctIPAddress,
			testHostName:  correctHostName,
			testTime:      time.Now().AddDate(10, 0, 1), // ~ 48 hours until expiry
			want:          true,
		},
	}
	t.Log("generating test certificate...")
	testCert, err := createTestCert([]net.IP{net.ParseIP(correctIPAddress)}, []string{correctHostName}, testCertExpiryTime)
	assert.Equal(t, nil, err)
	t.Log("successfully generated test certificate")
	for _, c := range testCases {
		t.Run(c.testName, func(t *testing.T) {
			got, err := kubeletCertificateNeedsRegeneration(c.testIPAddress, c.testHostName, testCert, c.testTime)
			assert.Equal(t, nil, err)
			assert.Equal(t, c.want, got)
		})
	}
}

// createTestCert creates a self-signed certificate for use in tests, it incorporates the given ipAddress's, hostNames, and expiry time
func createTestCert(ipAddress []net.IP, hostname []string, notAfter time.Time) (tls.Certificate, error) {
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2022),
		Subject: pkix.Name{
			Organization:  []string{"Rancher"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Green Pastures"},
			StreetAddress: []string{"123 Cattle Drive"},
			PostalCode:    []string{"94016"},
		},
		NotBefore:    time.Now(),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		SubjectKeyId: []byte{5, 4, 1, 6, 8},
		DNSNames:     hostname,
		IPAddresses:  ipAddress,
	}

	certPriv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return tls.Certificate{}, err
	}
	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return tls.Certificate{}, err
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, cert, &certPriv.PublicKey, privKey)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPem := new(bytes.Buffer)
	certPrivPem := new(bytes.Buffer)
	if err = pem.Encode(certPem, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}); err != nil {
		return tls.Certificate{}, err
	}
	if err = pem.Encode(certPrivPem, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPriv),
	}); err != nil {
		return tls.Certificate{}, err
	}

	return tls.X509KeyPair(certPem.Bytes(), certPrivPem.Bytes())
}
