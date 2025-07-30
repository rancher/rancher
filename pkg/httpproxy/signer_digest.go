package httpproxy

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rancher/norman/httperror"
)

func (d digest) sign(req *http.Request, secrets SecretGetter, auth string) error {
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
