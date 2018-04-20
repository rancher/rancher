package model

type Response struct {
	Status  int    `json:"status"`
	Message string `json:"msg"`
	Data    Domain `json:"data,omitempty"`
}
