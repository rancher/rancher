package awsauth

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
)

func prepareRequestV2(request *http.Request, keys Credentials) *http.Request {

	keyID := keys.AccessKeyID

	values := url.Values{}
	values.Set("AWSAccessKeyId", keyID)
	values.Set("SignatureVersion", "2")
	values.Set("SignatureMethod", "HmacSHA256")
	values.Set("Timestamp", timestampV2())

	augmentRequestQuery(request, values)

	if request.URL.Path == "" {
		request.URL.Path += "/"
	}

	return request
}

func stringToSignV2(request *http.Request) string {
	str := request.Method + "\n"
	str += strings.ToLower(request.URL.Host) + "\n"
	str += request.URL.Path + "\n"
	str += canonicalQueryStringV2(request)
	return str
}

func signatureV2(strToSign string, keys Credentials) string {
	hashed := hmacSHA256([]byte(keys.SecretAccessKey), strToSign)
	return base64.StdEncoding.EncodeToString(hashed)
}

func canonicalQueryStringV2(request *http.Request) string {
	return request.URL.RawQuery
}

func timestampV2() string {
	return now().Format(timeFormatV2)
}

const timeFormatV2 = "2006-01-02T15:04:05"
