package httpproxy

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/rancher/norman/httperror"
	v1 "github.com/rancher/rancher/pkg/types/apis/core/v1"
)

const (
	defaultAWSRegion = "us-east-1"
)

var requiredHeadersForAws = map[string]bool{"host": true,
	"x-amz-content-sha256": true,
	"x-amz-date":           true,
	"x-amz-user-agent":     true}

type Signer interface {
	sign(*http.Request, v1.SecretInterface, string) error
}

func newSigner(auth string) Signer {
	splitAuth := strings.Split(auth, " ")
	switch strings.ToLower(splitAuth[0]) {
	case "awsv4":
		return awsv4{}
	case "bearer":
		return bearer{}
	case "basic":
		return basic{}
	case "digest":
		return digest{}
	case "arbitrary":
		return arbitrary{}
	}
	return nil
}

func (br bearer) sign(req *http.Request, secrets v1.SecretInterface, auth string) error {
	data, secret, err := getAuthData(auth, secrets, []string{"passwordField", "credID"})
	if err != nil {
		return err
	}
	req.Header.Set(AuthHeader, fmt.Sprintf("%s %s", "Bearer", secret[data["passwordField"]]))
	return nil
}

func (b basic) sign(req *http.Request, secrets v1.SecretInterface, auth string) error {
	data, secret, err := getAuthData(auth, secrets, []string{"usernameField", "passwordField", "credID"})
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%s:%s", secret[data["usernameField"]], secret[data["passwordField"]])
	encoded := base64.URLEncoding.EncodeToString([]byte(key))
	req.Header.Set(AuthHeader, fmt.Sprintf("%s %s", "Basic", encoded))
	return nil
}

func (a awsv4) sign(req *http.Request, secrets v1.SecretInterface, auth string) error {
	_, secret, err := getAuthData(auth, secrets, []string{"credID"})
	if err != nil {
		return err
	}
	service, region := a.getServiceAndRegion(req.URL.Host)
	creds := credentials.NewStaticCredentials(secret["accessKey"], secret["secretKey"], "")
	awsSigner := v4.NewSigner(creds)
	var body []byte
	if req.Body != nil {
		body, err = ioutil.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("error reading request body %v", err)
		}
	}
	oldHeader, newHeader := http.Header{}, http.Header{}
	for header, value := range req.Header {
		if _, ok := requiredHeadersForAws[strings.ToLower(header)]; ok {
			newHeader[header] = value
		} else {
			oldHeader[header] = value
		}
	}
	req.Header = newHeader
	_, err = awsSigner.Sign(req, bytes.NewReader(body), service, region, time.Now())
	if err != nil {
		return err
	}
	for key, val := range oldHeader {
		req.Header.Add(key, strings.Join(val, ""))
	}
	return nil
}

func (a awsv4) getServiceAndRegion(host string) (string, string) {
	service := ""
	region := ""
	for _, partition := range endpoints.DefaultPartitions() {
		service, region = partitionServiceAndRegion(partition, host)
		// empty region is valid, but if one is found it should be assumed correct
		if region != "" {
			return service, region
		}
	}

	// if no region is found, global endpoint is assumed. In this case us-east-1 should be used for signing:
	// https://docs.aws.amazon.com/general/latest/gr/sigv4_elements.html
	return service, defaultAWSRegion
}

func partitionServiceAndRegion(partition endpoints.Partition, host string) (string, string) {
	service := ""
	partitionServices := partition.Services()
	for _, part := range strings.Split(host, ".") {
		if id := partitionServices[part].ID(); id != "" {
			service = id
			break
		}
	}

	if service == "" {
		return "", ""
	}

	host = strings.Trim(host, service)
	serviceRegions := partitionServices[service].Regions()
	for _, part := range strings.Split(host, ".") {
		if id := serviceRegions[part].ID(); id != "" {
			return service, id
		}
	}
	return service, ""
}

func (d digest) sign(req *http.Request, secrets v1.SecretInterface, auth string) error {
	data, secret, err := getAuthData(auth, secrets, []string{"usernameField", "passwordField", "credID"})
	if err != nil {
		return err
	}
	resp, err := doNewRequest(req) // request to get challenge fields from server
	if err != nil {
		return err
	}
	challengeData, err := parseChallenge(resp.Header.Get("WWW-Authenticate"))
	if err != nil {
		return err
	}
	challengeData["username"] = secret[data["usernameField"]]
	challengeData["password"] = secret[data["passwordField"]]
	signature, err := buildSignature(challengeData, req)
	if err != nil {
		return err
	}
	req.Header.Set(AuthHeader, fmt.Sprintf("%s %s", "Digest", signature))
	return nil
}

func doNewRequest(req *http.Request) (*http.Response, error) {
	newReq, err := http.NewRequest(req.Method, req.URL.String(), nil)
	if err != nil {
		return nil, err
	}
	newReq.Header.Set("Content-Type", "application/json")
	client := http.Client{}
	resp, err := client.Do(newReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != httperror.Unauthorized.Status {
		return nil, fmt.Errorf("expected 401 status code, got %v", resp.StatusCode)
	}
	resp.Body.Close()
	return resp, err
}

func parseChallenge(header string) (map[string]string, error) {
	if header == "" {
		return nil, fmt.Errorf("failed to get WWW-Authenticate header")
	}
	s := strings.Trim(header, " \n\r\t")
	if !strings.HasPrefix(s, "Digest ") {
		return nil, fmt.Errorf("bad challenge %s", header)
	}
	data := map[string]string{}
	s = strings.Trim(s[7:], " \n\r\t")
	terms := strings.Split(s, ", ")
	for _, term := range terms {
		splitTerm := strings.SplitN(term, "=", 2)
		data[splitTerm[0]] = strings.Trim(splitTerm[1], "\"")
	}
	return data, nil
}

func formResponse(qop string, data map[string]string, req *http.Request) (string, string) {
	hash1 := hash(fmt.Sprintf("%s:%s:%s", data["username"], data["realm"], data["password"]))
	hash2 := hash(fmt.Sprintf("%s:%s", req.Method, req.URL.Path))
	if qop == "" {
		return hash(fmt.Sprintf("%s:%s:%s", hash1, data["nonce"], hash2)), ""

	} else if qop == "auth" {
		cnonce := data["cnonce"]
		if cnonce == "" {
			cnonce = getCnonce()
		}
		return hash(fmt.Sprintf("%s:%s:%08x:%s:%s:%s",
			hash1, data["nonce"], 00000001, cnonce, qop, hash2)), cnonce
	}
	return "", ""
}

func buildSignature(data map[string]string, req *http.Request) (string, error) {
	qop, ok := data["qop"]
	if ok && qop != "auth" && qop != "" {
		return "", fmt.Errorf("qop not implemented %s", data["qop"])
	}
	response, cnonce := formResponse(qop, data, req)
	if response == "" {
		return "", fmt.Errorf("error forming response qop: %s", qop)
	}
	auth := []string{fmt.Sprintf(`username="%s"`, data["username"])}
	auth = append(auth, fmt.Sprintf(`realm="%s"`, data["realm"]))
	auth = append(auth, fmt.Sprintf(`nonce="%s"`, data["nonce"]))
	auth = append(auth, fmt.Sprintf(`uri="%s"`, req.URL.Path))
	auth = append(auth, fmt.Sprintf(`response="%s"`, response))
	if val, ok := data["opaque"]; ok && val != "" {
		auth = append(auth, fmt.Sprintf(`opaque="%s"`, data["opaque"]))
	}
	if qop != "" {
		auth = append(auth, fmt.Sprintf("qop=%s", qop))
		auth = append(auth, fmt.Sprintf("nc=%08x", 00000001))
		auth = append(auth, fmt.Sprintf("cnonce=%s", cnonce))
	}
	return strings.Join(auth, ", "), nil
}

func hash(field string) string {
	f := md5.New()
	f.Write([]byte(field))
	return hex.EncodeToString(f.Sum(nil))
}

func getCnonce() string {
	b := make([]byte, 8)
	io.ReadFull(rand.Reader, b)
	return fmt.Sprintf("%x", b)[:16]
}

func (a arbitrary) sign(req *http.Request, secrets v1.SecretInterface, auth string) error {
	data, _, err := getAuthData(auth, secrets, []string{})
	if err != nil {
		return err
	}
	splitHeaders := strings.Split(data["headers"], ",")
	for _, header := range splitHeaders {
		val := strings.SplitN(header, "=", 2)
		req.Header.Set(val[0], val[1])
	}
	return nil
}

type awsv4 struct{}

type bearer struct{}

type basic struct{}

type digest struct{}

type arbitrary struct{}
