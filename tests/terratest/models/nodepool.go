package models

type Nodepool struct {
	Quantity int    `json:"quantity"`
	Etcd     string `json:"etcd"`
	Cp       string `json:"cp"`
	Wkr      string `json:"wkr"`
}