package hosts

import (
	"fmt"
	"net"
)

func LocalHealthcheckFactory(h *Host) (func(network, address string) (net.Conn, error), error) {
	dialer := &dialer{
		host: h,
	}
	return dialer.DialHealthcheckLocally, nil
}

func (d *dialer) DialHealthcheckLocally(network, addr string) (net.Conn, error) {
	conn, err := net.Dial(network, addr)
	if err != nil {
		return nil, fmt.Errorf("Failed to dial address [%s]: %v", d.host.Address, err)
	}
	return conn, err
}
