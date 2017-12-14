package model

//AuthError structure contains the error resource definition
type AuthError struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
