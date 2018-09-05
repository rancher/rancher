package service

import (
	"net"
	"strconv"

	"fmt"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v3 "github.com/rancher/types/client/project/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func New(store types.Store) types.Store {
	return &Store{
		store,
	}
}

type Store struct {
	types.Store
}

func (p *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if schema.ID == "dnsRecord" {
		if convert.IsAPIObjectEmpty(data["hostname"]) {
			data["kind"] = "ClusterIP"
			data["clusterIp"] = nil
		} else {
			data["kind"] = "ExternalName"
			data["clusterIp"] = ""
		}
	}
	formatData(data)
	err := p.validateNonSpecialIP(schema, data)
	if err != nil {
		return nil, err
	}
	return p.Store.Create(apiContext, schema, data)
}

func formatData(data map[string]interface{}) {
	var ports []interface{}
	servicePort := v3.ServicePort{
		Port:       42,
		TargetPort: intstr.Parse(strconv.FormatInt(42, 10)),
		Protocol:   "TCP",
		Name:       "default",
	}
	m, err := convert.EncodeToMap(servicePort)
	if err != nil {
		logrus.Warnf("Failed to transform service port to map: %v", err)
		return
	}
	ports = append(ports, m)
	data["ports"] = ports
}

func (p *Store) validateNonSpecialIP(schema *types.Schema, data map[string]interface{}) error {
	if schema.ID == "dnsRecord" {
		ips := data["ipAddresses"]
		if ips != nil {
			for _, ip := range ips.([]interface{}) {
				IP := net.ParseIP(ip.(string))
				if IP == nil {
					return fmt.Errorf("%s must be a valid IP address", IP)
				}
				if IP.IsUnspecified() {
					return fmt.Errorf("%s may not be unspecified (0.0.0.0)", IP)
				}
				if IP.IsLoopback() {
					return fmt.Errorf("%s may not be in the loopback range (127.0.0.0/8)", IP)
				}
				if IP.IsLinkLocalUnicast() {
					return fmt.Errorf("%s may not be in the link-local range (169.254.0.0/16)", IP)
				}
				if IP.IsLinkLocalMulticast() {
					return fmt.Errorf("%s may not be in the link-local multicast range (224.0.0.0/24)", IP)
				}
			}
		}
	}
	return nil
}
