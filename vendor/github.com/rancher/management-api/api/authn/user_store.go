package authn

import (
	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/client/management/v3"
	"golang.org/x/crypto/bcrypt"
)

type userStore struct {
	types.Store
}

func SetUserStore(schema *types.Schema) {
	schema.Store = &userStore{schema.Store}
}

func hashPassword(data map[string]interface{}) error {
	pass, ok := data[client.UserFieldPassword].(string)
	if !ok {
		return errors.New("password not a string")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return errors.Wrap(err, "problem encrypting password")
	}
	data[client.UserFieldPassword] = string(hash)

	return nil
}

func (s *userStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if err := hashPassword(data); err != nil {
		return nil, err
	}

	created, err := s.Store.Create(apiContext, schema, data)
	if err != nil {
		return nil, err
	}

	if pids, ok := created[client.UserFieldPrincipalIDs].([]interface{}); ok {
		if id, ok := created[client.UserFieldId].(string); ok {
			created[client.UserFieldPrincipalIDs] = append(pids, "local://"+id)
			return s.Update(apiContext, schema, created, id)
		}
	}

	return created, err
}
