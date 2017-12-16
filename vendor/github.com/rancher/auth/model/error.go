package model

//AuthError structure contains the error resource definition
type AuthError struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message"`
}
