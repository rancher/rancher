package utils

import (
	"bytes"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"log/syslog"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"

	"github.com/pkg/errors"
	kafka "github.com/segmentio/kafka-go"
	"github.com/vmihailenco/msgpack"
)

const (
	sslv23  = "SSLv23"
	tlsv1   = "TLSv1"
	tlsv1_1 = "TLSv1_1"
	tlsv1_2 = "TLSv1_2"
)

const (
	letterHex       = "0123456789abcdef"
	deadlineTimeout = time.Duration(2 * time.Second)
)

var (
	testMessage              = "Rancher logging target setting validated"
	fluentdForwarderTestData = []byte(fmt.Sprintf(`["rancher",[[ %d, {"message": "`+testMessage+`"}]]]`, time.Now().Unix()))
	kafkaTestData            = kafka.Message{Value: []byte(testMessage)}
	httpTestData             = []byte(`{"event": "` + testMessage + `", "sourcetype": "rancher"}`)
)

type LoggingTargetTestWrap interface {
	TestReachable(dial dialer.Dialer, includeSendTestLog bool) error
}

func NewLoggingTargetTestWrap(loggingTargets v3.LoggingTargets) LoggingTargetTestWrap {
	if loggingTargets.ElasticsearchConfig != nil {
		return &elasticsearchTestWrap{loggingTargets.ElasticsearchConfig}
	} else if loggingTargets.SplunkConfig != nil {
		return &splunkTestWrap{loggingTargets.SplunkConfig}
	} else if loggingTargets.SyslogConfig != nil {
		return &syslogTestWrap{loggingTargets.SyslogConfig}
	} else if loggingTargets.KafkaConfig != nil {
		return &kafkaTestWrap{loggingTargets.KafkaConfig}
	} else if loggingTargets.FluentForwarderConfig != nil {
		return &fluentForwarderTestWrap{loggingTargets.FluentForwarderConfig}
	} else if loggingTargets.CustomTargetConfig != nil {
		return &customTargetTestWrap{loggingTargets.CustomTargetConfig}
	}

	return nil
}

type elasticsearchTestWrap struct {
	*v3.ElasticsearchConfig
}

type splunkTestWrap struct {
	*v3.SplunkConfig
}

type kafkaTestWrap struct {
	*v3.KafkaConfig
}

type syslogTestWrap struct {
	*v3.SyslogConfig
}

type fluentForwarderTestWrap struct {
	*v3.FluentForwarderConfig
}

type customTargetTestWrap struct {
	*v3.CustomTargetConfig
}

func (w *elasticsearchTestWrap) TestReachable(dial dialer.Dialer, includeSendTestLog bool) error {
	url, err := url.Parse(w.Endpoint)
	if err != nil {
		return errors.Wrapf(err, "parse url %s failed", w.Endpoint)
	}

	isTLS := url.Scheme == "https"
	var tlsConfig *tls.Config
	if isTLS {
		tlsConfig, err = buildTLSConfig(w.Certificate, w.ClientCert, w.ClientKey, w.ClientKeyPass, w.SSLVersion, url.Hostname(), w.SSLVerify)
		if err != nil {
			return errors.Wrap(err, "build tls config failed")
		}
	}

	if !includeSendTestLog {
		conn, err := newTCPConn(dial, url.Host, tlsConfig, true)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}

	index := getIndex(w.DateFormat, w.IndexPrefix)

	url.Path = path.Join(url.Path, index, "/container_log")

	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(httpTestData))
	if err != nil {
		return errors.Wrap(err, "create request failed")
	}
	req.Header.Set("Content-Type", "application/json")

	if w.AuthUserName != "" && w.AuthPassword != "" {
		req.SetBasicAuth(w.AuthUserName, w.AuthPassword)
	}

	return testReachableHTTP(dial, req, tlsConfig)
}

func (w *splunkTestWrap) TestReachable(dial dialer.Dialer, includeSendTestLog bool) error {
	url, err := url.Parse(w.Endpoint)
	if err != nil {
		return errors.Wrapf(err, "parse url %s failed", url)
	}

	isTLS := url.Scheme == "https"
	var tlsConfig *tls.Config
	if isTLS {
		tlsConfig, err = buildTLSConfig(w.Certificate, w.ClientCert, w.ClientKey, w.ClientKeyPass, "", url.Hostname(), w.SSLVerify)
		if err != nil {
			return errors.Wrap(err, "build tls config failed")
		}
	}

	if !includeSendTestLog {
		conn, err := newTCPConn(dial, url.Host, tlsConfig, true)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}

	url.Path = path.Join(url.Path, "/services/collector")
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(httpTestData))
	if err != nil {
		return errors.Wrap(err, "create request failed")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Splunk %s", w.Token))

	return testReachableHTTP(dial, req, tlsConfig)
}

func (w *kafkaTestWrap) TestReachable(dial dialer.Dialer, includeSendTestLog bool) error {
	if w.SaslUsername != "" && w.SaslPassword != "" {
		//TODO: Now we don't have a out of the box Kafka go client fit our request which both support sasl and could pass conn to it.
		//kafka-go has a PR to support sasl, but not merge yet due to the mantainer want support Negotiation and Kerberos as well, we will add test func to sasl after the sasl in kafka-go is stable
		return nil
	}

	if w.ZookeeperEndpoint != "" {
		url, err := url.Parse(w.ZookeeperEndpoint)
		if err != nil {
			return errors.Wrapf(err, "parse url %s failed", url)
		}

		var tlsConfig *tls.Config
		if url.Scheme == "https" {
			tlsConfig, err = buildTLSConfig(w.Certificate, w.ClientCert, w.ClientKey, "", "", url.Hostname(), true)
			if err != nil {
				return err
			}
		}

		conn, err := newTCPConn(dial, url.Host, tlsConfig, true)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}

	for _, v := range w.BrokerEndpoints {
		url, err := url.Parse(v)
		if err != nil {
			return errors.Wrapf(err, "parse url %s failed", url)
		}

		var tlsConfig *tls.Config
		if url.Scheme == "https" {
			tlsConfig, err = buildTLSConfig(w.Certificate, w.ClientCert, w.ClientKey, "", "", url.Hostname(), true)
			if err != nil {
				return err
			}
		}

		if includeSendTestLog {
			if err := w.sendData2Kafka(url.Host, dial, tlsConfig); err != nil {
				return err
			}
		}

	}
	return nil
}

func (w *syslogTestWrap) TestReachable(dial dialer.Dialer, includeSendTestLog bool) error {
	//TODO: for udp we can't use cluster dialer now, how to handle in cluster deploy syslog
	syslogTestData := newRFC5424Message(w.Severity, w.Program, w.Token, testMessage)
	if w.Protocol == "udp" {
		conn, err := newUDPConn(w.Endpoint)
		if err != nil {
			return errors.Wrapf(err, "dail to udp endpoint %s failed", w.Endpoint)
		}
		defer conn.Close()

		if includeSendTestLog {
			if _, err = conn.Write(syslogTestData); err != nil {
				return errors.Wrapf(err, "write to udp endpoint %s failed", w.Endpoint)
			}
		}
		return nil
	}

	var tlsConfig *tls.Config
	if w.EnableTLS {
		hostName, _, err := net.SplitHostPort(w.Endpoint)
		if err != nil {
			return errors.Wrapf(err, "parse endpoint %s failed", w.Endpoint)
		}

		tlsConfig, err = buildTLSConfig(w.Certificate, w.ClientCert, w.ClientKey, "", "", hostName, w.SSLVerify)
		if err != nil {
			return err
		}
	}

	conn, err := newTCPConn(dial, w.Endpoint, tlsConfig, true)
	if err != nil {
		return err
	}
	defer conn.Close()

	if !includeSendTestLog {
		return nil
	}

	if _, err = conn.Write(syslogTestData); err != nil {
		return errors.Wrapf(err, "write data to server %s failed", w.Endpoint)
	}

	if !w.EnableTLS {
		// for not tls try read to check whether the server close connect already
		resBuf := make([]byte, 1024)
		if _, err := conn.Read(resBuf); err != nil {
			return errors.Wrapf(err, "read data from syslog server %s failed", w.Endpoint)
		}
	}

	return nil
}

func (w *fluentForwarderTestWrap) TestReachable(dial dialer.Dialer, includeSendTestLog bool) error {
	var err error

	for _, s := range w.FluentServers {
		var tlsConfig *tls.Config
		if w.EnableTLS {
			serverName := s.Hostname
			if serverName == "" {
				host, _, err := net.SplitHostPort(s.Endpoint)
				if err != nil {
					return errors.Wrapf(err, "parse endpoint %s failed", s.Endpoint)
				}
				serverName = host
			}
			tlsConfig, err = buildTLSConfig(w.Certificate, "", "", "", "", serverName, false)
			if err != nil {
				return err
			}
		}

		conn, err := newTCPConn(dial, s.Endpoint, tlsConfig, true)
		if err != nil {
			return err
		}

		if !includeSendTestLog {
			conn.Close()
			continue
		}

		if err := w.sendData2Server(conn, s.SharedKey, s.Username, s.Password, s.Endpoint); err != nil {
			conn.Close()
			return err
		}

		conn.Close()
	}
	return nil
}

func (w *fluentForwarderTestWrap) sendData2Server(conn net.Conn, shareKey, username, password, endpoint string) error {
	if shareKey == "" && username == "" && password == "" {
		if _, err := conn.Write(fluentdForwarderTestData); err != nil {
			return errors.Wrapf(err, "write data to server %s failed", endpoint)
		}
	}

	buf := make([]byte, 1024)
	if _, err := conn.Read(buf); err != nil && err != io.EOF {
		return errors.Wrapf(err, "read data from fluent forwarder server %s failed", endpoint)
	}

	var heloBody []interface{}
	if err := msgpack.Unmarshal(buf, &heloBody); err != nil {
		return errors.Wrapf(err, "use msgpack to unmashal helo from %s failed", endpoint)
	}

	if len(heloBody) < 2 {
		return errors.New("helo body from fluentd don't have enough info")
	}

	var heloOption heloOption
	if err := convert.ToObj(heloBody[1], &heloOption); err != nil {
		return errors.Wrapf(err, "convert helo body from %s failed", endpoint)
	}

	nonce, err := base64.StdEncoding.DecodeString(heloOption.Nonce)
	if err != nil {
		return errors.Wrapf(err, "decode nonce from %s failed", endpoint)
	}

	auth, err := base64.StdEncoding.DecodeString(heloOption.Auth)
	if err != nil {
		return errors.Wrapf(err, "decode auth from %s failed", endpoint)
	}

	ping, err := w.generateFluentForwarderPing(shareKey, string(nonce), username, password, string(auth))
	if err != nil {
		return errors.Wrap(err, "generate fluent forwarder ping failed")
	}

	if _, err = conn.Write([]byte(ping)); err != nil {
		return errors.Wrap(err, "write ping info to fluent forwarder failed")
	}

	if _, err = conn.Write(fluentdForwarderTestData); err != nil {
		return errors.Wrap(err, "write test data to fluent forwarder failed")
	}

	pongBuf := make([]byte, 1024)
	if _, err = conn.Read(pongBuf); err != nil && err != io.EOF {
		return errors.Wrapf(err, "read pong data from fluent forwarder server %s failed", endpoint)
	}

	return w.decodeFluentForwarderPong(pongBuf)
}

func (w *customTargetTestWrap) TestReachable(dial dialer.Dialer, includeSendTestLog bool) error {
	return nil
}

type heloOption struct {
	Nonce     string `json:"nonce"`
	Auth      string `json:"auth"`
	Keepalive bool   `json:"keepalive"`
}

func (w *fluentForwarderTestWrap) generateFluentForwarderPing(shareKey, nonce, username, password, auth string) (string, error) {
	// format from fluentd code: ['PING', self_hostname, shared_key_salt, sha512_hex(shared_key_salt + self_hostname + nonce + shared_key), username || '', sha512_hex(auth_salt + username + password) || '']
	hostname, err := os.Hostname()
	if err != nil {
		return "", errors.Wrap(err, "get host failed")
	}

	salt := randHex(16)
	fullSharedKey := fmt.Sprintf("%s%s%s%s", salt, hostname, nonce, shareKey)
	hash := sha512.New()
	hash.Write([]byte(fullSharedKey))
	sharedKeyHex := hex.EncodeToString(hash.Sum(nil))

	passwordHex := ""
	if auth != "" {
		fullPassword := fmt.Sprintf("%s%s%s", auth, username, password)
		passwordHash := sha512.New()
		passwordHash.Write([]byte(fullPassword))
		passwordHex = hex.EncodeToString(passwordHash.Sum(nil))
	}
	return fmt.Sprintf(`["PING", "%s", "%s", "%s", "%s", "%s"]`, hostname, salt, sharedKeyHex, username, passwordHex), nil
}

func randHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterHex[rand.Intn(len(letterHex))]
	}
	return string(b)
}

func (w *fluentForwarderTestWrap) decodeFluentForwarderPong(pong []byte) error {
	// format from fluentd code ['PONG', bool(authentication result), 'reason if authentication failed', self_hostname, sha512_hex(salt + self_hostname + nonce + sharedkey)]
	// sample:  ["PONG",false,"shared_key mismatch","",""]
	pongMsg := string(pong)
	pongMsg = strings.TrimPrefix(pongMsg, "[")
	pongMsg = strings.TrimSuffix(pongMsg, "]")
	pongMsgArray := strings.Split(pongMsg, ",")
	if len(pongMsgArray) != 5 {
		return errors.New("invalid format pong msg from fluentd, " + pongMsg)
	}

	if pongMsgArray[1] == "false" {
		return errors.New("auth failed from fluentd, pong response: " + pongMsgArray[2])
	}

	return nil
}

func testReachableHTTP(dial dialer.Dialer, req *http.Request, tlsConfig *tls.Config) error {
	transport := &http.Transport{
		Dial:            dial,
		TLSClientConfig: tlsConfig,
	}

	client := http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "request to logging target %s failed", req.URL)
	}

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return errors.Wrapf(err, "read response body from %s failed, response code is %v", req.URL.String(), res.StatusCode)
		}
		return fmt.Errorf("response code from %s is %v, not include in the 2xx success HTTP status codes, response body: %s", req.URL.String(), res.StatusCode, string(body))
	}

	return nil
}

func newTCPConn(dialer dialer.Dialer, smartHost string, tlsConfig *tls.Config, handshake bool) (net.Conn, error) {
	conn, err := dialer("tcp", smartHost)
	if err != nil {
		return nil, errors.Wrap(err, "create raw conn failed")
	}
	conn.SetDeadline(time.Now().Add(deadlineTimeout))

	if tlsConfig == nil {
		return conn, nil
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if handshake {
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return nil, errors.Wrapf(err, "tls handshake %s failed", smartHost)
		}
	}
	return tlsConn, nil
}

func newUDPConn(smartHost string) (net.Conn, error) {
	conn, err := net.Dial("udp", smartHost)
	if err != nil {
		return nil, errors.Wrapf(err, "dail to udp endpoint %s failed", smartHost)
	}
	conn.SetDeadline(time.Now().Add(deadlineTimeout))
	return conn, nil
}

func (w *kafkaTestWrap) sendData2Kafka(smartHost string, dial dialer.Dialer, tlsConfig *tls.Config) error {
	leaderConn, err := w.kafkaConn(dial, tlsConfig, smartHost)
	if err != nil {
		return err
	}
	defer leaderConn.Close()

	if _, err := leaderConn.WriteMessages(kafkaTestData); err != nil {
		return errors.Wrap(err, "write test message to kafka failed")
	}

	return nil
}

func (w *kafkaTestWrap) kafkaConn(dial dialer.Dialer, config *tls.Config, smartHost string) (*kafka.Conn, error) {
	defaultPartition := 0

	conn, err := newTCPConn(dial, smartHost, config, false)
	if err != nil {
		return nil, err
	}

	kafkaConn := kafka.NewConn(conn, w.Topic, defaultPartition)
	topicConf := kafka.TopicConfig{
		Topic:             w.Topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	}

	if err := kafkaConn.CreateTopics(topicConf); err != nil {
		kafkaConn.Close()
		return nil, err
	}

	partitions, err := kafkaConn.ReadPartitions(w.Topic)
	if err != nil {
		kafkaConn.Close()
		return nil, err
	}

	var leader kafka.Broker
	for _, v := range partitions {
		if v.ID == defaultPartition {
			leader = v.Leader
		}
	}

	leaderSmartHost := fmt.Sprintf("%s:%d", leader.Host, leader.Port)

	if leaderSmartHost == smartHost {
		return kafkaConn, nil
	}

	kafkaConn.Close()

	LeaderConn, err := newTCPConn(dial, leaderSmartHost, config, false)
	if err != nil {
		return nil, err
	}

	return kafka.NewConn(LeaderConn, w.Topic, defaultPartition), nil
}

func decodePEM(clientKey, passphrase string) ([]byte, error) {
	clientKeyBytes := []byte(clientKey)
	pemBlock, _ := pem.Decode(clientKeyBytes)
	if pemBlock == nil {
		return nil, fmt.Errorf("no valid private key found")
	}

	var err error
	if x509.IsEncryptedPEMBlock(pemBlock) {
		clientKeyBytes, err = x509.DecryptPEMBlock(pemBlock, []byte(passphrase))
		if err != nil {
			return nil, errors.Wrap(err, "private key is encrypted, but could not decrypt it")
		}
		clientKeyBytes = pem.EncodeToMemory(&pem.Block{Type: pemBlock.Type, Bytes: clientKeyBytes})
	}

	return clientKeyBytes, nil
}

func buildTLSConfig(rootCA, clientCert, clientKey, clientKeyPass, sslVersion, serverName string, sslVerify bool) (config *tls.Config, err error) {
	if rootCA == "" && clientCert == "" && clientKey == "" && clientKeyPass == "" && sslVersion == "" {
		return nil, nil
	}

	config = &tls.Config{
		InsecureSkipVerify: !sslVerify,
		ServerName:         serverName,
	}

	var decodeClientKeyBytes = []byte(clientKey)
	if clientKeyPass != "" {
		decodeClientKeyBytes, err = decodePEM(clientKey, clientKeyPass)
		if err != nil {
			return nil, err
		}
	}

	if clientCert != "" {
		cert, err := tls.X509KeyPair([]byte(clientCert), decodeClientKeyBytes)
		if err != nil {
			return nil, errors.Wrap(err, "load client cert and key failed")
		}

		config.Certificates = []tls.Certificate{cert}
	}

	if rootCA != "" {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM([]byte(rootCA))

		config.RootCAs = caCertPool
	}

	if sslVersion != "" {
		switch sslVersion {
		case sslv23:
			config.MaxVersion = tls.VersionSSL30
		case tlsv1:
			config.MaxVersion = tls.VersionTLS10
			config.MinVersion = tls.VersionTLS10
		case tlsv1_1:
			config.MaxVersion = tls.VersionTLS11
			config.MinVersion = tls.VersionTLS11
		case tlsv1_2:
			config.MaxVersion = tls.VersionTLS12
			config.MinVersion = tls.VersionTLS12
		}
	}

	return config, nil
}

func getIndex(dateFormat, prefix string) string {
	var index string
	today := time.Now()
	switch dateFormat {
	case "YYYY":
		index = fmt.Sprintf("%s-%04d", prefix, today.Year())
	case "YYYY-MM":
		index = fmt.Sprintf("%s-%04d-%02d", prefix, today.Year(), today.Month())
	case "YYYY-MM-DD":
		index = fmt.Sprintf("%s-%04d-%02d-%02d", prefix, today.Year(), today.Month(), today.Day())
	}
	return index
}

func newRFC5424Message(severityStr, app, token, msg string) []byte {
	if app == "" {
		app = "rancher"
	}

	severity := getSeverity(severityStr)
	syslogVersion := 1
	timestamp := time.Now().Format(time.RFC3339)
	msgID := randHex(6)
	hostname, _ := os.Hostname()

	return []byte(fmt.Sprintf("<%d>%v %v %v %v %v %v [%v] %v\n",
		severity,
		syslogVersion,
		timestamp,
		hostname,
		app,
		os.Getpid(),
		msgID,
		token,
		msg,
	))
}

func getSeverity(severityStr string) syslog.Priority {
	severityMap := map[string]syslog.Priority{
		"emerg":   syslog.LOG_EMERG,
		"alert":   syslog.LOG_ALERT,
		"crit":    syslog.LOG_CRIT,
		"err":     syslog.LOG_ERR,
		"warning": syslog.LOG_WARNING,
		"notice":  syslog.LOG_NOTICE,
		"info":    syslog.LOG_INFO,
		"debug":   syslog.LOG_DEBUG,
	}

	if severity, ok := severityMap[severityStr]; ok {
		return severity
	}

	return syslog.LOG_INFO
}
