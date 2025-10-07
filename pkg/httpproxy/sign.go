package httpproxy

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
)

type SecretGetter func(namespace, name string) (*v1.Secret, error)

type Signer interface {
	sign(*http.Request, SecretGetter, string) error
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

func (br bearer) sign(req *http.Request, secrets SecretGetter, auth string) error {
	data, secret, err := getAuthData(auth, secrets, []string{"passwordField", "credID"})
	if err != nil {
		return err
	}
	req.Header.Set(AuthHeader, fmt.Sprintf("%s %s", "Bearer", secret[data["passwordField"]]))
	return nil
}

func (b basic) sign(req *http.Request, secrets SecretGetter, auth string) error {
	data, secret, err := getAuthData(auth, secrets, []string{"usernameField", "passwordField", "credID"})
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%s:%s", secret[data["usernameField"]], secret[data["passwordField"]])
	encoded := base64.URLEncoding.EncodeToString([]byte(key))
	req.Header.Set(AuthHeader, fmt.Sprintf("%s %s", "Basic", encoded))
	return nil
}

func (a arbitrary) sign(req *http.Request, secrets SecretGetter, auth string) error {
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
