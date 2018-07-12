package hosts

import (
	"fmt"
	"net"
	"strconv"
)

const (
	DINDPort = "2375"
)

type dindDialer struct {
	Address string
	Port    string
	Network string
}

func DindConnFactory(h *Host) (func(network, address string) (net.Conn, error), error) {
	newDindDialer := &dindDialer{
		Address: h.Address,
		Port:    DINDPort,
	}
	return newDindDialer.Dial, nil
}

func DindHealthcheckConnFactory(h *Host) (func(network, address string) (net.Conn, error), error) {
	newDindDialer := &dindDialer{
		Address: h.Address,
		Port:    strconv.Itoa(h.LocalConnPort),
	}
	return newDindDialer.Dial, nil
}

func (d *dindDialer) Dial(network, addr string) (net.Conn, error) {
	conn, err := net.Dial(network, d.Address+":"+d.Port)
	if err != nil {
		return nil, fmt.Errorf("Failed to dial dind address [%s]: %v", addr, err)
	}
	return conn, err
}
