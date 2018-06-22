package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
	k8stypes "k8s.io/apimachinery/pkg/types"
	utilnet "k8s.io/apimachinery/pkg/util/net"
)

const (
	contentTypeJSON = "application/json"
)

const (
	levelNull = iota
	levelMetadata
	levelRequest
	levelRequestResponse
)

var (
	auditLevel = map[int]string{
		levelMetadata:        "MetadataLevel",
		levelRequest:         "RequestLevel",
		levelRequestResponse: "RequestResponseLevel",
	}

	bodyMethods = map[string]bool{
		http.MethodPut:  true,
		http.MethodPost: true,
	}
)

const (
	requestReceived  = "RequestReceived"
	responseComplete = "ResponseComplete"
)

type Log struct {
	AuditID        k8stypes.UID `json:"auditID,omitempty"`
	RequestURI     string       `json:"requestURI,omitempty"`
	RequestBody    interface{}  `json:"requestBody,omitempty"`
	ResponseBody   interface{}  `json:"responseBody,omitempty"`
	ResponseStatus string       `json:"responseStatus,omitempty"`
	SourceIPs      []string     `json:"sourceIPs,omitempty"`
	User           *userInfo    `json:"user,omitempty"`
	UserAgent      string       `json:"userAgent,omitempty"`
	Verb           string       `json:"verb,omitempty"`
	Stage          string       `json:"stage,omitempty"`
	StageTimestamp string       `json:"stageTimestamp,omitempty"`
}

type userInfo struct {
	Name  string   `json:"name,omitempty"`
	Group []string `json:"group,omitempty"`
}

type LogWriter struct {
	Level  int
	Output *lumberjack.Logger
}

func NewLogWriter(path string, level, maxAge, maxBackup, maxSize int) *LogWriter {
	if path == "" || level == levelNull {
		return nil
	}

	return &LogWriter{
		Level: level,
		Output: &lumberjack.Logger{
			Filename:   path,
			MaxAge:     maxAge,
			MaxBackups: maxBackup,
			MaxSize:    maxSize,
		},
	}
}

func (a *LogWriter) LogRequest(req *http.Request, auditID k8stypes.UID, authed bool, contentType, user string, group []string) error {
	al := &Log{
		AuditID:        auditID,
		Stage:          requestReceived,
		StageTimestamp: time.Now().Format("2006-01-02 15:04:05 -0700"),
		RequestURI:     req.RequestURI,
		Verb:           req.Method,
	}

	ips := utilnet.SourceIPs(req)
	var sourceIPs []string
	for i := range ips {
		sourceIPs = append(sourceIPs, ips[i].String())
	}
	al.SourceIPs = sourceIPs

	if authed {
		al.User = &userInfo{
			Name:  user,
			Group: group,
		}
	}

	var buffer bytes.Buffer

	alByte, err := json.Marshal(al)
	if err != nil {
		return err
	}
	buffer.Write(bytes.TrimRight(alByte, "}\n"))

	if a.Level >= levelRequest && bodyMethods[req.Method] && contentType == contentTypeJSON {
		reqBody, err := readBodyWithoutLosingContent(req)
		if err != nil {
			return err
		}
		buffer.WriteString(`,"requestBody":`)
		buffer.Write(bytes.TrimRight(reqBody, "\n"))
	}

	buffer.WriteString("}\n")
	_, err = a.Output.Write(buffer.Bytes())
	return err
}

func (a *LogWriter) LogResponse(resBody []byte, auditID k8stypes.UID, statusCode int, contentType string) error {
	if a.Level < levelRequestResponse {
		return nil
	}

	al := &Log{
		AuditID:        auditID,
		Stage:          responseComplete,
		StageTimestamp: time.Now().Format("2006-01-02 15:04:05 -0700"),
		ResponseStatus: fmt.Sprint(statusCode),
	}

	var buffer bytes.Buffer
	alByte, err := json.Marshal(al)
	if err != nil {
		return err
	}

	buffer.Write(bytes.TrimRight(alByte, "}\n"))

	if contentType == contentTypeJSON {
		buffer.WriteString(`,"responseBody":`)
		buffer.Write(bytes.TrimRight(resBody, "\n"))
	}

	buffer.WriteString("}\n")

	_, err = a.Output.Write(buffer.Bytes())
	return err
}

func readBodyWithoutLosingContent(req *http.Request) ([]byte, error) {
	if !bodyMethods[req.Method] {
		return nil, nil
	}

	bodyBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	return bodyBytes, nil
}
