package clustermanager

import (
	"github.com/rancher/norman/types"
	"github.com/sirupsen/logrus"
)

func (m *Manager) Expire(apiContext *types.APIContext, schema *types.Schema) {
	ac, err := m.getAccessControl(apiContext, schema)
	if err != nil {
		return
	}
	if e, ok := ac.(types.Expire); ok {
		e.Expire(apiContext, schema)
	}
}

func (m *Manager) CanCreate(apiContext *types.APIContext, schema *types.Schema) error {
	ac, err := m.getAccessControl(apiContext, schema)
	if err != nil {
		return err
	}
	return ac.CanCreate(apiContext, schema)
}

func (m *Manager) CanList(apiContext *types.APIContext, schema *types.Schema) error {
	ac, err := m.getAccessControl(apiContext, schema)
	if err != nil {
		return err
	}
	return ac.CanList(apiContext, schema)
}

func (m *Manager) CanGet(apiContext *types.APIContext, schema *types.Schema) error {
	ac, err := m.getAccessControl(apiContext, schema)
	if err != nil {
		return err
	}
	return ac.CanGet(apiContext, schema)
}

func (m *Manager) CanUpdate(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	ac, err := m.getAccessControl(apiContext, schema)
	if err != nil {
		return err
	}
	return ac.CanUpdate(apiContext, obj, schema)
}

func (m *Manager) CanDelete(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	ac, err := m.getAccessControl(apiContext, schema)
	if err != nil {
		return err
	}
	return ac.CanDelete(apiContext, obj, schema)
}

func (m *Manager) CanDo(apiGroup, resource, verb string, apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	ac, err := m.getAccessControl(apiContext, schema)
	if err != nil {
		return err
	}
	return ac.CanDo(apiGroup, resource, verb, apiContext, obj, schema)
}

func (m *Manager) Filter(apiContext *types.APIContext, schema *types.Schema, obj map[string]interface{}, context map[string]string) map[string]interface{} {
	ac, err := m.getAccessControl(apiContext, schema)
	if err != nil {
		logrus.Warnf("failed to find access control: %v", err)
		return nil
	}

	return ac.Filter(apiContext, schema, obj, context)
}

func (m *Manager) FilterList(apiContext *types.APIContext, schema *types.Schema, obj []map[string]interface{}, context map[string]string) []map[string]interface{} {
	ac, err := m.getAccessControl(apiContext, schema)
	if err != nil {
		logrus.Warnf("failed to find access control: %v", err)
		return nil
	}
	return ac.FilterList(apiContext, schema, obj, context)
}

func (m *Manager) getAccessControl(apiContext *types.APIContext, schema *types.Schema) (types.AccessControl, error) {
	return m.AccessControl(apiContext, getContext(schema))
}

func getContext(schema *types.Schema) types.StorageContext {
	if schema == nil || schema.Store == nil {
		return types.DefaultStorageContext
	}
	return schema.Store.Context()
}
