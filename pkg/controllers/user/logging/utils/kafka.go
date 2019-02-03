package utils

import (
	"crypto/tls"
	"fmt"
	"net/url"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
	kafka "github.com/segmentio/kafka-go"
)

var (
	kafkaTestData = kafka.Message{Value: []byte(testMessage)}
)

type kafkaTestWrap struct {
	*v3.KafkaConfig
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
