package cert

import (
	"time"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/cert"
	"github.com/rancher/types/client/project/v3"
)

type Store struct {
	types.Store
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if err := AddCertInfo(data); err != nil {
		return nil, err
	}

	return s.Store.Create(apiContext, schema, data)
}

func (s *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	if err := AddCertInfo(data); err != nil {
		return nil, err
	}

	return s.Store.Update(apiContext, schema, data, id)
}

func AddCertInfo(data map[string]interface{}) error {
	certs, _ := data[client.CertificateFieldCerts].(string)
	key, _ := data[client.CertificateFieldKey].(string)

	if certs == "" || key == "" {
		return nil
	}

	certInfo, err := cert.Info(certs, key)
	if err != nil {
		return httperror.NewFieldAPIError(httperror.InvalidBodyContent, "certs", err.Error())
	}

	certData, err := convert.EncodeToMap(certInfo)
	if err != nil {
		return err
	}

	for k, v := range certData {
		if t, ok := v.(time.Time); ok {
			data[k] = convert.ToString(t)
		} else {
			data[k] = v
		}
	}

	return nil
}
