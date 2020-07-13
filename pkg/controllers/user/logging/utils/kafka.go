package utils

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config/dialer"
	kafka "github.com/segmentio/kafka-go"
)

const (
	defaultPartition = 0
)

type kafkaTestWrap struct {
	*v3.KafkaConfig
}

func getKafkaTestData() kafka.Message {
	return kafka.Message{
		Value: []byte(time.Now().Format(time.RFC1123Z) + " " + testMessage),
		Time:  time.Now(),
	}
}
func (w *kafkaTestWrap) TestReachable(ctx context.Context, dial dialer.Dialer, includeSendTestLog bool) error {
	if w.SaslUsername != "" && w.SaslPassword != "" {
		//TODO: Now we don't have a out of the box Kafka go client fit our request which both support sasl and could pass conn to it.
		//kafka-go has a PR to support sasl, but not merge yet due to the mantainer want support Negotiation and Kerberos as well, we will add test func to sasl after the sasl in kafka-go is stable
		return nil
	}

	if w.ZookeeperEndpoint != "" {
		url, err := url.Parse(w.ZookeeperEndpoint)
		if err != nil {
			return errors.Wrapf(err, "couldn't parse url %s", w.ZookeeperEndpoint)
		}

		var tlsConfig *tls.Config
		if url.Scheme == "https" {
			tlsConfig, err = buildTLSConfig(w.Certificate, w.ClientCert, w.ClientKey, "", "", url.Hostname(), true)
			if err != nil {
				return err
			}
		}

		conn, err := newTCPConn(ctx, dial, url.Host, tlsConfig, true)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}

	if len(w.BrokerEndpoints) == 0 {
		return errors.New("broker endpoint is empty")
	}

	broker0URL, err := url.Parse(w.BrokerEndpoints[0])
	if err != nil {
		return errors.Wrapf(err, "couldn't parse url %s", w.BrokerEndpoints[0])
	}

	var broker0TLSConfig *tls.Config
	if broker0URL.Scheme == "https" {
		if broker0TLSConfig, err = buildTLSConfig(w.Certificate, w.ClientCert, w.ClientKey, "", "", broker0URL.Hostname(), true); err != nil {
			return err
		}
	}

	broker0Host := broker0URL.Host
	broker0Conn, err := w.kafkaConn(ctx, dial, broker0TLSConfig, broker0Host)
	if err != nil {
		return err
	}
	defer broker0Conn.Close()

	brokers, err := broker0Conn.Brokers()
	if err != nil {
		return errors.Wrap(err, "couldn't get broker list")
	}

	topicConf := kafka.TopicConfig{
		Topic:             w.Topic,
		NumPartitions:     len(brokers),
		ReplicationFactor: 1,
	}

	brokerController, err := broker0Conn.Controller()
	if err != nil {
		return errors.Wrap(err, "couldn't get broker controller")
	}

	brokerControllerHost := brokerHost(brokerController.Host, brokerController.Port)
	if brokerControllerHost == broker0Host {
		if err := broker0Conn.CreateTopics(topicConf); err != nil {
			return errors.Wrapf(err, "couldn't create topic %s", w.Topic)
		}

		if includeSendTestLog {
			if err = w.sendData2Kafka(ctx, broker0Conn, dial); err != nil {
				return err
			}
		}
	} else {
		var brokerControllerTLSConfig *tls.Config
		if broker0URL.Scheme == "https" {
			err := w.verifyKafkaCert()
			if err != nil {
				return err
			}

			if brokerControllerTLSConfig, err = buildTLSConfig(w.Certificate, w.ClientCert, w.ClientKey, "", "", brokerController.Host, true); err != nil {
				return err
			}
		}

		brokerControllerConn, err := w.kafkaConn(ctx, dial, brokerControllerTLSConfig, brokerControllerHost)
		if err != nil {
			return err
		}
		defer brokerControllerConn.Close()

		if err := brokerControllerConn.CreateTopics(topicConf); err != nil {
			return errors.Wrapf(err, "couldn't create topic %s", w.Topic)
		}

		if includeSendTestLog {
			if err = w.sendData2Kafka(ctx, brokerControllerConn, dial); err != nil {
				return err
			}
		}
	}

	brokerMap := make(map[string]kafka.Broker)
	for _, v := range brokers {
		host := brokerHost(v.Host, v.Port)
		brokerMap[host] = v
	}

	for _, v := range w.BrokerEndpoints[1:] {
		endpointURL, err := url.Parse(v)
		if err != nil {
			return errors.Wrapf(err, "couldn't parse url %s", v)
		}
		if endpointURL.Host == brokerControllerHost || endpointURL.Host == broker0Host {
			continue
		}

		if _, ok := brokerMap[endpointURL.Host]; !ok {
			return errors.New(v + " isn't included in broker list")
		}

		if err = w.checkEndpointReachable(ctx, endpointURL, dial); err != nil {
			return err
		}
	}
	return nil
}

func (w *kafkaTestWrap) checkEndpointReachable(ctx context.Context, endpointURL *url.URL, dial dialer.Dialer) error {
	var tlsConfig *tls.Config
	var err error
	if endpointURL.Scheme == "https" {
		err := w.verifyKafkaCert()
		if err != nil {
			return err
		}

		tlsConfig, err = buildTLSConfig(w.Certificate, w.ClientCert, w.ClientKey, "", "", endpointURL.Hostname(), true)
		if err != nil {
			return err
		}
	}

	conn, err := w.kafkaConn(ctx, dial, tlsConfig, endpointURL.Host)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Controller()
	if err != nil {
		return errors.Wrapf(err, "couldn't get response from %s", endpointURL.Host)

	}
	return nil
}

func (w *kafkaTestWrap) getKafkaPartitionLeader(kafkaConn *kafka.Conn) (*kafka.Broker, error) {
	partitions, err := kafkaConn.ReadPartitions(w.Topic)
	if err != nil {
		return nil, errors.Wrap(wrapErrEOF(err), "couldn't read kafka partitions")
	}
	var patitionLeader kafka.Broker
	for _, v := range partitions {
		if v.ID == defaultPartition {
			patitionLeader = v.Leader
		}
	}

	if &patitionLeader == nil {
		return nil, errors.New("couldn't get topic partition leader")
	}
	return &patitionLeader, nil
}

func (w *kafkaTestWrap) sendData2Kafka(ctx context.Context, kafkaConn *kafka.Conn, dial dialer.Dialer) error {
	patitionLeader, err := w.getKafkaPartitionLeader(kafkaConn)
	if err != nil {
		return err
	}

	patitionLeaderHost := brokerHost(patitionLeader.Host, patitionLeader.Port)
	if patitionLeaderHost == kafkaConn.RemoteAddr().String() {
		if _, err := kafkaConn.WriteMessages(getKafkaTestData()); err != nil {
			return errors.Wrap(err, "couldn't write test message to kafka")
		}
		return nil
	}

	tlsConfig, err := buildTLSConfig(w.Certificate, w.ClientCert, w.ClientKey, "", "", patitionLeader.Host, true)
	if err != nil {
		return err
	}

	patitionLeaderConn, err := w.kafkaConn(ctx, dial, tlsConfig, patitionLeaderHost)
	if err != nil {
		return err
	}
	defer patitionLeaderConn.Close()
	if _, err := patitionLeaderConn.WriteMessages(getKafkaTestData()); err != nil {
		return errors.Wrap(err, "couldn't write test message to kafka")
	}

	return nil
}

func (w *kafkaTestWrap) kafkaConn(ctx context.Context, dial dialer.Dialer, config *tls.Config, smartHost string) (*kafka.Conn, error) {
	conn, err := newTCPConn(ctx, dial, smartHost, config, false)
	if err != nil {
		return nil, err
	}

	return kafka.NewConn(conn, w.Topic, defaultPartition), nil
}

func (w *kafkaTestWrap) verifyKafkaCert() error {
	isSelfSigned, err := IsSelfSigned([]byte(w.Certificate))
	if err != nil {
		return err
	}

	if isSelfSigned && IsClientAuthEnaled(w.ClientCert, w.ClientKey) {
		return errors.New("Certificate verification failed, Kafka doesn't support self-signed certificate when client authentication is enabled")
	}
	return nil
}

func wrapErrEOF(err error) error {
	if err == io.EOF {
		return errors.New("unexpected EOF, connection closed by remote server")
	}

	return err
}

func brokerHost(hostName string, port int) string {
	return fmt.Sprintf("%s:%d", hostName, port)
}
