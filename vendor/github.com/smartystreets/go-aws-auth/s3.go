package awsauth

import (
	"encoding/base64"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

func signatureS3(stringToSign string, keys Credentials) string {
	hashed := hmacSHA1([]byte(keys.SecretAccessKey), stringToSign)
	return base64.StdEncoding.EncodeToString(hashed)
}

func stringToSignS3(request *http.Request) string {
	str := request.Method + "\n"

	if request.Header.Get("Content-Md5") != "" {
		str += request.Header.Get("Content-Md5")
	} else {
		body := readAndReplaceBody(request)
		if len(body) > 0 {
			str += hashMD5(body)
		}
	}
	str += "\n"

	str += request.Header.Get("Content-Type") + "\n"

	if request.Header.Get("Date") != "" {
		str += request.Header.Get("Date")
	} else {
		str += timestampS3()
	}

	str += "\n"

	canonicalHeaders := canonicalAmzHeadersS3(request)
	if canonicalHeaders != "" {
		str += canonicalHeaders
	}

	str += canonicalResourceS3(request)

	return str
}

func stringToSignS3Url(method string, expire time.Time, path string) string {
	return method + "\n\n\n" + timeToUnixEpochString(expire) + "\n" + path
}

func timeToUnixEpochString(t time.Time) string {
	return strconv.FormatInt(t.Unix(), 10)
}

func canonicalAmzHeadersS3(request *http.Request) string {
	var headers []string

	for header := range request.Header {
		standardized := strings.ToLower(strings.TrimSpace(header))
		if strings.HasPrefix(standardized, "x-amz") {
			headers = append(headers, standardized)
		}
	}

	sort.Strings(headers)

	for i, header := range headers {
		headers[i] = header + ":" + strings.Replace(request.Header.Get(header), "\n", " ", -1)
	}

	if len(headers) > 0 {
		return strings.Join(headers, "\n") + "\n"
	} else {
		return ""
	}
}

func canonicalResourceS3(request *http.Request) string {
	res := ""

	if isS3VirtualHostedStyle(request) {
		bucketname := strings.Split(request.Host, ".")[0]
		res += "/" + bucketname
	}

	res += request.URL.Path

	for _, subres := range strings.Split(subresourcesS3, ",") {
		if strings.HasPrefix(request.URL.RawQuery, subres) {
			res += "?" + subres
		}
	}

	return res
}

func prepareRequestS3(request *http.Request) *http.Request {
	request.Header.Set("Date", timestampS3())
	if request.URL.Path == "" {
		request.URL.Path += "/"
	}
	return request
}

// Info: http://docs.aws.amazon.com/AmazonS3/latest/dev/VirtualHosting.html
func isS3VirtualHostedStyle(request *http.Request) bool {
	service, _ := serviceAndRegion(request.Host)
	return service == "s3" && strings.Count(request.Host, ".") == 3
}

func timestampS3() string {
	return now().Format(timeFormatS3)
}

const (
	timeFormatS3   = time.RFC1123Z
	subresourcesS3 = "acl,lifecycle,location,logging,notification,partNumber,policy,requestPayment,torrent,uploadId,uploads,versionId,versioning,versions,website"
)
