package audit

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/rancher/rancher/pkg/data/management"
	"github.com/stretchr/testify/suite"
)

var errAny = errors.New("any error is allowed")

type AuditTest struct {
	suite.Suite
}

func TestAuditSuite(t *testing.T) {
	suite.Run(t, new(AuditTest))
}
func (a *AuditTest) TestConcealSensitiveData() {
	r, err := constructKeyConcealRegex()
	a.Require().NoError(err, "failed compiling sanitizing regex")
	logger := auditLog{
		log:                nil,
		writer:             nil,
		reqBody:            nil,
		keysToConcealRegex: r,
	}

	machineDataInput, machineDataWant := []byte("{"), []byte("{")
	for _, v := range management.DriverData {
		for key, value := range v {
			if strings.HasPrefix(key, "optional") {
				continue
			}
			public := strings.HasPrefix(key, "public")
			for _, item := range value {
				machineDataInput = append(machineDataInput, []byte("\""+item+"\" : \"fake_"+item+"\",")...)
				if public {
					machineDataWant = append(machineDataWant, []byte("\""+item+"\" : \"fake_"+item+"\",")...)
				} else {
					machineDataWant = append(machineDataWant, []byte("\""+item+"\" : \""+redacted+"\",")...)
				}
			}
		}
	}
	machineDataInput[len(machineDataInput)-1] = byte('}')
	machineDataWant[len(machineDataWant)-1] = byte('}')

	tests := []struct {
		name  string
		uri   string
		input []byte
		want  []byte
	}{
		{
			name:  "password entry",
			input: []byte(`{"password": "fake_password", "user": "fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"password":"%s","user":"fake_user"}`, redacted)),
		},
		{
			name:  "Password entry",
			input: []byte(`{"Password": "fake_password", "user": "fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"Password":"%s","user":"fake_user"}`, redacted)),
		},
		{
			name:  "password entry no space",
			input: []byte(`{"password":"whatever you want", "user": "fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"password":"%s", "user": "fake_user"}`, redacted)),
		},
		{
			name:  "Password entry no space",
			input: []byte(`{"Password":"A whole bunch of \"\"}{()","user":"fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"Password":"%s", "user": "fake_user"}`, redacted)),
		},
		{
			name:  "currentPassword entry",
			input: []byte(`{"currentPassword": "something super secret", "user": "fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"currentPassword": "%s", "user": "fake_user"}`, redacted)),
		},
		{
			name:  "newPassword entry",
			input: []byte(`{"newPassword": "don't share this", "user": "fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"newPassword": "%s", "user": "fake_user"}`, redacted)),
		},
		{
			name:  "Multiple password entries",
			input: []byte(`{"currentPassword": "fake_password", "newPassword": "new_fake_password", "user": "fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"currentPassword": "%s", "newPassword": "%[1]s", "user": "fake_user"}`, redacted)),
		},
		{
			name:  "No password entries",
			input: []byte(`{"user": "fake_user", "user_info": "some information about the user", "request_info": "some info about the request"}`),
			want:  []byte(`{"user": "fake_user", "user_info": "some information about the user", "request_info": "some info about the request"}`),
		},
		{
			name:  "Strategic password examples",
			input: []byte(`{"anotherPassword":"\"password\"","currentPassword":"password\":","newPassword":"newPassword\\\":","shortPassword":"'","user":"fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"anotherPassword":"%s","currentPassword":"%[1]s","newPassword":"%[1]s","shortPassword":"%[1]s","user":"fake_user"}`, redacted)),
		},
		{
			name:  "Token entry",
			input: []byte(`{"accessToken": "fake_access_token", "user": "fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"accessToken": "%s", "user": "fake_user"}`, redacted)),
		},
		{
			name:  "Token entry in slice",
			input: []byte(`[{"accessToken": "fake_access_token", "user": "fake_user"}]`),
			want:  []byte(fmt.Sprintf(`[{"accessToken": "%s", "user": "fake_user"}]`, redacted)),
		},
		{
			name:  "With public fields",
			input: []byte(`{"accessKey": "fake_access_key", "secretKey": "fake_secret_key", "user": "fake_user"}`),
			want:  []byte(fmt.Sprintf(`{"accessKey": "fake_access_key", "secretKey": "%s", "user": "fake_user"}`, redacted)),
		},
		{
			name:  "With secret data",
			input: []byte(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}, "accessToken" : "fake_access_token"}`),
			want:  []byte(fmt.Sprintf(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"%s", "bar": "%[1]s"}, "accessToken" : "%[1]s"}`, redacted)),
			uri:   "/secrets",
		},
		{
			name:  "With secret data and wrong URI",
			input: []byte(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}, "accessToken" : "fake_access_token"}`),
			want:  []byte(fmt.Sprintf(`{"type":"Opaque","metadata":{"namespace":"default","name":"my secret"},"_type":"Opaque","data":{"foo":"c3VwZXIgc2VjcmV0IGRhdGE=\\", "bar": "U3VwZXIgU2VjcmV0IERhdGEK"}, "accessToken" : "%s"}`, redacted)),
			uri:   "/not-secret",
		},
		{
			name:  "With nested sensitive information",
			input: []byte(`{"sensitiveData": {"accessToken": "fake_access_token", "user": "fake_user"}}`),
			want:  []byte(fmt.Sprintf(`{"sensitiveData": {"accessToken": "%s", "user": "fake_user"}}`, redacted)),
		},
		{
			name:  "With all machine driver fields",
			input: machineDataInput,
			want:  machineDataWant,
		},
	}
	for i := range tests {
		test := tests[i]
		a.Run(test.name, func() {
			fmt.Println(string(test.want[0]))
			fmt.Println(byte('['))
			if len(test.want) > 0 && test.want[0] == byte('[') {
				var want []interface{}
				err := json.Unmarshal(test.want, &want)
				a.NoError(err, "failed to unmarshal")
				got := logger.concealSensitiveData(test.uri, test.input)
				var gotMap []interface{}
				err = json.Unmarshal(got, &gotMap)
				a.NoError(err, "failed to unmarshal")
				a.Equal(want, gotMap, "concealSensitiveData() for slice = %s, want %s", got, test.want)
				return
			}
			var want map[string]interface{}
			err := json.Unmarshal(test.want, &want)
			a.NoError(err, "failed to unmarshal")
			got := logger.concealSensitiveData(test.uri, test.input)
			var gotMap map[string]interface{}
			err = json.Unmarshal(got, &gotMap)
			a.NoError(err, "failed to unmarshal")
			a.Equal(want, gotMap, "concealSensitiveData() for map = %s, want %s", got, test.want)
		})
	}
}
func (a *AuditTest) TestCompression() {
	// Create a temp log file
	tmpFile, err := os.CreateTemp("", "audit-test")
	a.Require().NoError(err, "Failed to create temp directory.")
	// close the file so the logger can open it for writing
	err = tmpFile.Close()
	a.Require().NoError(err, "Failed to close temporary file after creation")

	tmpPath := tmpFile.Name()
	defer func() {
		err = os.RemoveAll(tmpPath)
		a.NoError(err, "Failed to clean up temp directory")
	}()

	writer := NewLogWriter(tmpPath, LevelRequestResponse, 30, 30, 100)
	a.Require().NotNil(writer, "Failed to create auditWriter.")

	sensitiveRegex, err := regexp.Compile(`[pP]assword|[tT]oken`)
	a.Require().NoErrorf(err, "Failed to create valid regex: %v", err)

	req, err := http.NewRequest(http.MethodGet, "/test", nil)
	a.Require().NoErrorf(err, "Failed to create request: %v", err)

	auditLog, err := newAuditLog(writer, req, sensitiveRegex)
	a.Require().NoErrorf(err, "Failed to create AuditLog: %v", err)

	const testString = "{\"test\":\"response\"}"
	const testString2 = "{\"test\":\"request\"}"

	tests := []struct {
		name             string
		respHeader       http.Header
		respBody         []byte
		reqBody          []byte
		returnCode       int
		expectedRespBody string
		expectedReqBody  string
		level            Level
		Error            error
	}{
		{
			name:       "invalid Encoding",
			respHeader: http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"bzip2"}},
			respBody:   []byte(testString),
			Error:      ErrUnsupportedEncoding,
			level:      LevelRequestResponse,
		},
		{
			name:             "none encoding",
			respHeader:       http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}},
			respBody:         []byte(testString),
			reqBody:          []byte(testString2),
			expectedRespBody: testString,
			expectedReqBody:  testString2,
			level:            LevelRequestResponse,
		},
		{
			name:            "request only",
			respHeader:      http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}},
			respBody:        []byte(testString),
			reqBody:         []byte(testString2),
			expectedReqBody: testString2,
			level:           LevelRequest,
		},
		{
			name:       "meta only",
			respHeader: http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}},
			respBody:   []byte(testString),
			reqBody:    []byte(testString2),
			level:      LevelMetadata,
		},
		{
			name:             "gzip encoding",
			respHeader:       http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"gzip"}},
			respBody:         a.gzip(testString),
			expectedRespBody: testString,
			level:            LevelRequestResponse,
		},
		{
			name:             "deflate encoding",
			respHeader:       http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"deflate"}},
			respBody:         a.deflate(testString),
			expectedRespBody: testString,
			level:            LevelRequestResponse,
		},
		{
			name:             "empty gzip response",
			respHeader:       http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"gzip"}},
			respBody:         a.gzip("{}"),
			expectedRespBody: "{}",
			level:            LevelRequestResponse,
		},
		{
			name:             "empty deflate response",
			respHeader:       http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"deflate"}},
			respBody:         a.deflate("{}"),
			expectedRespBody: "{}",
			level:            LevelRequestResponse,
		},

		{
			name:             "empty response",
			respHeader:       http.Header{"Content-Type": []string{"application/json"}},
			respBody:         []byte(""),
			expectedRespBody: "",
			level:            LevelRequestResponse,
		},
		{
			name:       "invalid gzip response",
			respHeader: http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"gzip"}},
			respBody:   []byte(testString),
			Error:      errAny,
			level:      LevelRequestResponse,
		},
		{
			name:       "invalid deflate response",
			respHeader: http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"deflate"}},
			respBody:   []byte(testString),
			Error:      errAny,
			level:      LevelRequestResponse,
		},
		{
			name:       "invalid json gzip response",
			respHeader: http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"gzip"}},
			respBody:   a.gzip(""),
			Error:      &json.SyntaxError{},
			level:      LevelRequestResponse,
		},
		{
			name:       "invalid json deflate response",
			respHeader: http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"deflate"}},
			respBody:   a.deflate("Bad Data[]}"),
			Error:      &json.SyntaxError{},
			level:      LevelRequestResponse,
		},
	}

	for i := range tests {
		test := tests[i]
		a.Run(test.name, func() {
			writer.Level = test.level
			auditLog.reqBody = []byte(test.reqBody)
			// write the test to the audit logger
			err := auditLog.write(nil, req.Header, test.respHeader, test.returnCode, test.respBody)

			// if we are expecting an error check the error is not nil and the correct type
			if test.Error != nil {
				if errors.Is(test.Error, errAny) {
					a.Error(err, "Expected an Error")
					return
				}
				a.Truef(errorIsType(err, test.Error), "Received error does not wrap an error of type '%T'", test.Error)
				return
			}
			a.Require().NoErrorf(err, "Failed to write log: %v.", err)

			// validate the json written to the file is as expected\

			expectedData := a.addMeta(auditLog.log, nil, test.respHeader, test.expectedReqBody, test.expectedRespBody)

			a.JSONEqf(expectedData, a.drain(tmpPath), "Incorrect JSON stored.")
		})
	}

}

func (a *AuditTest) TestFilterSensitiveHeader() {
	// Create a temp log file
	tmpFile, err := os.CreateTemp("", "audit-test")
	a.Require().NoError(err, "Failed to create temp directory.")
	// close the file so the logger can open it for writing
	err = tmpFile.Close()
	a.Require().NoError(err, "Failed to close temporary file after creation")

	tmpPath := tmpFile.Name()
	defer func() {
		err = os.RemoveAll(tmpPath)
		a.NoError(err, "Failed to clean up temp directory")
	}()

	writer := NewLogWriter(tmpPath, LevelRequestResponse, 30, 30, 100)
	a.Require().NotNil(writer, "Failed to create auditWriter.")

	sensitiveRegex, err := regexp.Compile(`[pP]assword|[tT]oken`)
	a.Require().NoErrorf(err, "Failed to create valid regex: %v", err)

	req, err := http.NewRequest(http.MethodGet, "/test", nil)
	a.Require().NoErrorf(err, "Failed to create request: %v", err)

	auditLog, err := newAuditLog(writer, req, sensitiveRegex)
	a.Require().NoErrorf(err, "Failed to create AuditLog: %v", err)

	tests := []struct {
		name               string
		respHeader         http.Header
		reqHeader          http.Header
		expectedRespHeader http.Header
		expectedReqHeader  http.Header
	}{
		{
			name:               "sensitive request header: \"X-Api-Tunnel-Param\"",
			reqHeader:          http.Header{"X-Api-Tunnel-Params": []string{"abcd"}},
			respHeader:         http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}},
			expectedRespHeader: http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}},
		},
		{
			name:               "sensitive request header: \"X-Api-Tunnel-Token\"",
			reqHeader:          http.Header{"X-Api-Tunnel-Token": []string{"abcd"}},
			respHeader:         http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}},
			expectedRespHeader: http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}},
		},
		{
			name:               "sensitive request header: \"Authorization\"",
			reqHeader:          http.Header{"Authorization": []string{"abcd"}},
			respHeader:         http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}},
			expectedRespHeader: http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}},
		},
		{
			name:               "sensitive request header: \"Cookie\"",
			reqHeader:          http.Header{"Cookie": []string{"abcd"}},
			respHeader:         http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}},
			expectedRespHeader: http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}},
		},
		{
			name:               "non-sensitive request header and sensitive request header: \"Cookie\"",
			reqHeader:          http.Header{"Cookie": []string{"abcd"}, "User-Agent": []string{"useragent1"}},
			respHeader:         http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}},
			expectedRespHeader: http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}},
			expectedReqHeader:  http.Header{"User-Agent": []string{"useragent1"}},
		},
		{
			name:               "sensitive response header: \"Cookie\"",
			respHeader:         http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}, "Cookie": []string{"abcd"}},
			expectedRespHeader: http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}},
		},
		{
			name:               "sensitive response header: \"Set-Cookie\"",
			respHeader:         http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}, "Set-Cookie": []string{"abcd"}},
			expectedRespHeader: http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"none"}},
		},
	}
	writer.Level = LevelMetadata
	for i := range tests {
		test := tests[i]
		a.Run(test.name, func() {
			writer.Level = 1
			// write the test to the audit logger
			auditLog.log.RequestHeader = test.reqHeader
			err := auditLog.write(nil, test.reqHeader, test.respHeader, 0, []byte{})

			a.Require().NoErrorf(err, "Failed to write log: %v.", err)

			// validate the json written to the file is as expected\

			expectedData := a.addMeta(auditLog.log, test.expectedReqHeader, test.expectedRespHeader, "", "")

			a.JSONEqf(expectedData, a.drain(tmpPath), "Incorrect JSON stored.")
		})
	}
}

// addMeta adds expected log metadata to the expected log message.
func (a *AuditTest) addMeta(log *log, reqHeader, respHeader http.Header, reqBody, respBody string) string {
	data := map[string]interface{}{}
	if reqBody != "" {
		reqBodyData := map[string]interface{}{}
		err := json.Unmarshal([]byte(reqBody), &reqBodyData)
		a.NoErrorf(err, "Failed to unmarshal test body data: %v", err)
		data["requestBody"] = reqBodyData
	}
	if respBody != "" {
		respBodyData := map[string]interface{}{}
		err := json.Unmarshal([]byte(respBody), &respBodyData)
		a.NoErrorf(err, "Failed to unmarshal test body data: %v", err)
		data["responseBody"] = respBodyData
	}

	data["method"] = log.Method
	data["requestTimestamp"] = log.RequestTimestamp
	data["auditID"] = log.AuditID
	data["responseHeader"] = respHeader
	if reqHeader != nil {
		data["requestHeader"] = reqHeader
	}
	data["responseTimestamp"] = log.ResponseTimestamp
	retJSON, err := json.Marshal(data)
	a.NoErrorf(err, "Failed to add json metadata for log message check: %v", err)
	return string(retJSON)
}

// read a file's content then truncate
func (a *AuditTest) drain(tmpFile string) string {
	data, err := os.ReadFile(tmpFile)
	a.NoErrorf(err, "Failed to read the temp file")
	err = os.Truncate(tmpFile, 0)
	a.NoError(err, "Failed to truncate temp file")
	return string(data)
}

// gzip the given string
func (a *AuditTest) gzip(input string) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.Write([]byte(input))
	a.Require().NoError(w.Close())
	return buf.Bytes()
}

// deflate the given string
func (a *AuditTest) deflate(input string) []byte {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	w.Write([]byte(input))
	a.Require().NoError(w.Close())
	return buf.Bytes()
}

func errorIsType(err, target error) bool {
	targetType := reflect.TypeOf(target)
	for err != nil {
		if reflect.TypeOf(err) == targetType {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}
