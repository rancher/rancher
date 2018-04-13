package awsauth

import (
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
)

func hashedCanonicalRequestV4(request *http.Request, meta *metadata) string {
	// TASK 1. http://docs.aws.amazon.com/general/latest/gr/sigv4-create-canonical-request.html

	payload := readAndReplaceBody(request)
	payloadHash := hashSHA256(payload)
	request.Header.Set("X-Amz-Content-Sha256", payloadHash)

	// Set this in header values to make it appear in the range of headers to sign
	request.Header.Set("Host", request.Host)

	var sortedHeaderKeys []string
	for key, _ := range request.Header {
		switch key {
		case "Content-Type", "Content-Md5", "Host":
		default:
			if !strings.HasPrefix(key, "X-Amz-") {
				continue
			}
		}
		sortedHeaderKeys = append(sortedHeaderKeys, strings.ToLower(key))
	}
	sort.Strings(sortedHeaderKeys)

	var headersToSign string
	for _, key := range sortedHeaderKeys {
		value := strings.TrimSpace(request.Header.Get(key))
		if key == "host" {
			//AWS does not include port in signing request.
			if strings.Contains(value, ":") {
				split := strings.Split(value, ":")
				port := split[1]
				if port == "80" || port == "443" {
					value = split[0]
				}
			}
		}
		headersToSign += key + ":" + value + "\n"
	}
	meta.signedHeaders = concat(";", sortedHeaderKeys...)
	canonicalRequest := concat("\n", request.Method, normuri(request.URL.Path), normquery(request.URL.Query()), headersToSign, meta.signedHeaders, payloadHash)

	return hashSHA256([]byte(canonicalRequest))
}

func stringToSignV4(request *http.Request, hashedCanonReq string, meta *metadata) string {
	// TASK 2. http://docs.aws.amazon.com/general/latest/gr/sigv4-create-string-to-sign.html

	requestTs := request.Header.Get("X-Amz-Date")

	meta.algorithm = "AWS4-HMAC-SHA256"
	meta.service, meta.region = serviceAndRegion(request.Host)
	meta.date = tsDateV4(requestTs)
	meta.credentialScope = concat("/", meta.date, meta.region, meta.service, "aws4_request")

	return concat("\n", meta.algorithm, requestTs, meta.credentialScope, hashedCanonReq)
}

func signatureV4(signingKey []byte, stringToSign string) string {
	// TASK 3. http://docs.aws.amazon.com/general/latest/gr/sigv4-calculate-signature.html

	return hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
}

func prepareRequestV4(request *http.Request) *http.Request {
	necessaryDefaults := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded; charset=utf-8",
		"X-Amz-Date":   timestampV4(),
	}

	for header, value := range necessaryDefaults {
		if request.Header.Get(header) == "" {
			request.Header.Set(header, value)
		}
	}

	if request.URL.Path == "" {
		request.URL.Path += "/"
	}

	return request
}

func signingKeyV4(secretKey, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	return kSigning
}

func buildAuthHeaderV4(signature string, meta *metadata, keys Credentials) string {
	credential := keys.AccessKeyID + "/" + meta.credentialScope

	return meta.algorithm +
		" Credential=" + credential +
		", SignedHeaders=" + meta.signedHeaders +
		", Signature=" + signature
}

func timestampV4() string {
	return now().Format(timeFormatV4)
}

func tsDateV4(timestamp string) string {
	return timestamp[:8]
}

const timeFormatV4 = "20060102T150405Z"
