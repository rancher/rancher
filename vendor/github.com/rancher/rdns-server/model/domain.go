package model

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Domain struct {
	Fqdn       string     `json:"fqdn,omitempty"`
	Hosts      []string   `json:"hosts,omitempty"`
	Expiration *time.Time `json:"expiration,omitempty"`
}

func (d *Domain) String() string {
	return fmt.Sprintf("{Fqdn: %s, Hosts: %s, Expiration: %s}", d.Fqdn, d.Hosts, d.Expiration.String())
}

type DomainOptions struct {
	Fqdn  string   `json:"fqdn"`
	Hosts []string `json:"hosts"`
}

func (d *DomainOptions) String() string {
	return fmt.Sprintf("{Fqdn: %s, Hosts: %s}", d.Fqdn, d.Hosts)
}

func ParseDomainOptions(r *http.Request) (*DomainOptions, error) {
	var opts DomainOptions
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&opts)
	return &opts, err
}
