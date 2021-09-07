package user

import (
	"os"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
)

var Password string = os.Getenv("RANCHER_PASSWORD")

type UserOps struct {
	v3.UserOperations
}

func NewUser(client *v3.Client) *UserOps {
	return &UserOps{
		client.User,
	}
}

// CreateUser is a function that creates a User using a Client object with a specified *v3.User
func (u *UserOps) CreateUser(username, password, displayname string, changepassword bool) (*v3.User, error) {
	enabled := true
	return u.Create(&v3.User{
		Username:           username,
		Password:           password,
		Name:               displayname,
		MustChangePassword: changepassword,
		Enabled:            &enabled,
	})
}

// GetUser is a function that gets a specific *v3.User using a Client object with a specified id
func (u *UserOps) GetUser(id string) (*v3.User, error) {

	collection, err := u.List(&types.ListOpts{
		Filters: map[string]interface{}{
			"id": id,
		},
	})

	if err != nil {
		return nil, err
	}

	user := collection.Data[0]

	return &user, nil
}
