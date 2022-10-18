package user

import (
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name       string
		username   string
		password   string
		expectsErr bool
	}{
		{
			name:       "password too short",
			username:   "admin",
			password:   "tooshort",
			expectsErr: true,
		},
		{
			name:       "username equals password min length",
			username:   "passwordpass",
			password:   "passwordpass",
			expectsErr: true,
		},
		{
			name:       "username and password almost match",
			username:   "administrator",
			password:   "administrator1",
			expectsErr: false,
		},
		{
			name:       "12 byte password, 6 runes",
			username:   "admin",
			password:   "пароль",
			expectsErr: true,
		},
		{
			name:       "23 byte password, 12 runes",
			username:   "admin",
			password:   "абвгдеёжзий1",
			expectsErr: false,
		},
		{
			name:       "username equals password min length unicode",
			username:   "абвгдеёжзий1",
			password:   "абвгдеёжзий1",
			expectsErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePassword(tt.username, tt.password, 12)
			if err != nil && !tt.expectsErr {
				t.Errorf("Received unexpected error: %v", err)
			} else if err == nil && tt.expectsErr {
				t.Error("Expected error when non received")
			}
		})
	}

}
