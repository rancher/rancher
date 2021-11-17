package client


	


import (
	
)

const (
    RotateEncryptionKeyOutputType = "rotateEncryptionKeyOutput"
	RotateEncryptionKeyOutputFieldMessage = "message"
)

type RotateEncryptionKeyOutput struct {
        Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

