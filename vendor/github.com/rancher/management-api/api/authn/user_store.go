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

	if id, ok := created[types.ResourceFieldID].(string); ok {
		var principalIDs []interface{}
		if pids, ok := created[client.UserFieldPrincipalIDs].([]interface{}); ok {
			principalIDs = pids
		}
		created[client.UserFieldPrincipalIDs] = append(principalIDs, "local://"+id)
		return s.Update(apiContext, schema, created, id)
	}

	return created, err
}

func (s *userStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {

	req := apiContext.Request

	schemaData, err := s.Store.List(apiContext, schema, opt)
	if err != nil {
		return nil, err
	}
	userID := req.Header.Get("Impersonate-User")
	if userID != "" {
		for _, data := range schemaData {
			id, ok := data[types.ResourceFieldID].(string)
			if ok {
				if id == userID {
					data["me"] = "true"
				}
			}
		}
	}
	return schemaData, nil
}
