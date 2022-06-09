package client

const (
	UserStatusType            = "userStatus"
	UserStatusFieldConditions = "conditions"
)

type UserStatus struct {
	Conditions []UserCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
