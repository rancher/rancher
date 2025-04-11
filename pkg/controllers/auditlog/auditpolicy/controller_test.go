package auditpolicy

import (
	"io"
	"testing"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/controllers/auditlog/auditpolicy/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultNamespace   = "cattle-system"
	testHandlerFuncKey = "cattle-system/auditpolicy"
)

var (
	samplePolicy = auditlogv1.AuditPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultNamespace,
			Name:      "test",
		},
		Spec: auditlogv1.AuditPolicySpec{
			Enabled: true,
			Filters: []auditlogv1.Filter{
				{
					Action:     auditlogv1.FilterActionDeny,
					RequestURI: ".*",
				},
			},
			AdditionalRedactions: []auditlogv1.Redaction{
				{
					Headers: []string{
						".*[tT]oken.*",
					},
					Paths: []string{
						"$.some.path.to.value",
						"$..[Password,password]",
					},
				},
			},
		},
		Status: auditlogv1.AuditPolicyStatus{},
	}
)

func setup(t *testing.T, level auditlogv1.Level) handler {
	writer, err := audit.NewWriter(io.Discard, audit.WriterOptions{
		DefaultPolicyLevel: level,
	})
	if err != nil {
		t.Error("failed to create writer for audit log handler: %w", err)
	}

	h := handler{
		auditpolicy: &fake.MockController{
			Objs: map[string]map[string]auditlogv1.AuditPolicy{
				defaultNamespace: map[string]auditlogv1.AuditPolicy{},
			},
		},
		writer: writer,
	}

	return h
}

func TestOnChangeAddSimplePolicy(t *testing.T) {
	policy := samplePolicy

	h := setup(t, auditlogv1.LevelHeaders)

	_, err := h.auditpolicy.Create(&policy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionTrue, "")

	actual, err := h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	expectedPolicy, err := audit.PolicyFromAuditPolicy(&policy)
	assert.NoError(t, err)

	actualPolicy, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.True(t, ok)
	assert.Equal(t, expectedPolicy, actualPolicy)
}

func TestOnChangeAddInvalidPolicyFilterRequestURIRegex(t *testing.T) {
	policy := samplePolicy
	policy.Spec.Filters = []auditlogv1.Filter{
		{
			Action:     auditlogv1.FilterActionAllow,
			RequestURI: "*",
		},
	}

	h := setup(t, auditlogv1.LevelHeaders)

	_, err := h.auditpolicy.Create(&policy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeValid, metav1.ConditionFalse, "failed to create filter: failed to compile regex '*': error parsing regexp: missing argument to repetition operator: `*`")

	actual, err := h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)
}

func TestOnChangeAddInvalidPolicyFilterAction(t *testing.T) {
	policy := samplePolicy
	policy.Spec.Filters = []auditlogv1.Filter{
		{
			Action: "don't not allow",
		},
	}

	h := setup(t, auditlogv1.LevelHeaders)

	_, err := h.auditpolicy.Create(&policy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeValid, metav1.ConditionFalse, "failed to create filter: invalid filter action: 'don't not allow'")

	actual, err := h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)
}

func TestOnChangeAddInvalidPolicyRedactorHeaderRegex(t *testing.T) {
	policy := samplePolicy
	policy.Spec.AdditionalRedactions = []auditlogv1.Redaction{
		{
			Headers: []string{
				"*",
			},
		},
	}

	h := setup(t, auditlogv1.LevelHeaders)

	_, err := h.auditpolicy.Create(&policy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeValid, metav1.ConditionFalse, "failed to create redactor: failed to compile headers regexes: failed to compile regex: error parsing regexp: missing argument to repetition operator: `*`")

	actual, err := h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)
}

func TestOnChangeAddInvalidPolicyRedactorJSONPath(t *testing.T) {
	policy := samplePolicy
	policy.Spec.AdditionalRedactions = []auditlogv1.Redaction{
		{
			Paths: []string{
				".missing.root",
			},
		},
	}

	h := setup(t, auditlogv1.LevelHeaders)

	_, err := h.auditpolicy.Create(&policy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeValid, metav1.ConditionFalse, "failed to create redactor: failed to parse paths: failed to parse jsonpath: paths must begin with the root object identifier: '$'")

	actual, err := h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)
}

func TestOnChangeAddDisablePolicy(t *testing.T) {
	policy := samplePolicy
	policy.Spec.Enabled = false

	h := setup(t, auditlogv1.LevelHeaders)

	_, err := h.auditpolicy.Create(&policy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionFalse, "")

	actual, err := h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)
}

func TestOnChangeDisableActivePolicy(t *testing.T) {
	policy := samplePolicy

	h := setup(t, auditlogv1.LevelHeaders)

	_, err := h.auditpolicy.Create(&policy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionTrue, "")

	actual, err := h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	policy.Spec.Enabled = false

	_, err = h.auditpolicy.Update(&policy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected.Spec.Enabled = false
	updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionFalse, "")

	actual, err = h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)
}

func TestOnChangeOverwriteActiveWithInvalid(t *testing.T) {
	policy := samplePolicy

	h := setup(t, auditlogv1.LevelHeaders)

	_, err := h.auditpolicy.Create(&policy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionTrue, "")

	actual, err := h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	expectedPolicy, err := audit.PolicyFromAuditPolicy(&policy)
	assert.NoError(t, err)

	actualPolicy, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.True(t, ok)
	assert.Equal(t, expectedPolicy, actualPolicy)

	invalidPolicy := policy
	invalidPolicy.Spec.Filters = []auditlogv1.Filter{
		{
			Action:     auditlogv1.FilterActionAllow,
			RequestURI: "*",
		},
	}

	_, err = h.auditpolicy.Update(&invalidPolicy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &invalidPolicy)
	assert.NoError(t, err)

	expected = invalidPolicy
	updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeValid, metav1.ConditionFalse, "failed to create filter: failed to compile regex '*': error parsing regexp: missing argument to repetition operator: `*`")

	actual, err = h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	expectedPolicy, err = audit.PolicyFromAuditPolicy(&policy)
	assert.NoError(t, err)

	actualPolicy, ok = h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.True(t, ok)
	assert.Equal(t, expectedPolicy, actualPolicy)
}

func TestOnChangeOverwriteInvalidWithActive(t *testing.T) {
	invalidPolicy := samplePolicy
	invalidPolicy.Spec.Filters = []auditlogv1.Filter{
		{
			Action:     auditlogv1.FilterActionAllow,
			RequestURI: "*",
		},
	}

	h := setup(t, auditlogv1.LevelHeaders)

	_, err := h.auditpolicy.Create(&invalidPolicy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &invalidPolicy)
	assert.NoError(t, err)

	expected := invalidPolicy
	updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeValid, metav1.ConditionFalse, "failed to create filter: failed to compile regex '*': error parsing regexp: missing argument to repetition operator: `*`")

	actual, err := h.auditpolicy.Get(invalidPolicy.Namespace, invalidPolicy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, ok := h.writer.GetPolicy(invalidPolicy.Namespace, invalidPolicy.Name)
	assert.False(t, ok)

	policy := samplePolicy

	_, err = h.auditpolicy.Update(&policy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected = policy
	updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionFalse, "")

	actual, err = h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	expectedPolicy, err := audit.PolicyFromAuditPolicy(&policy)
	assert.NoError(t, err)

	actualPolicy, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.True(t, ok)
	assert.Equal(t, expectedPolicy, actualPolicy)
}

func TestOnRemoveActivePolicy(t *testing.T) {
	policy := samplePolicy

	h := setup(t, auditlogv1.LevelHeaders)

	_, err := h.auditpolicy.Create(&policy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionTrue, "")

	actual, err := h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, err = h.OnRemove(testHandlerFuncKey, &policy)
	assert.NoError(t, err)
}

func TestOnRemoveDisabledPolicy(t *testing.T) {
	policy := samplePolicy
	policy.Spec.Enabled = false

	h := setup(t, auditlogv1.LevelHeaders)

	_, err := h.auditpolicy.Create(&policy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionFalse, "")

	actual, err := h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)

	_, err = h.OnRemove(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	_, ok = h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)
}

func TestOnRemoveInvalidPolicy(t *testing.T) {
	policy := samplePolicy
	policy.Spec.Filters = []auditlogv1.Filter{
		{
			Action:     auditlogv1.FilterActionAllow,
			RequestURI: "*",
		},
	}

	h := setup(t, auditlogv1.LevelHeaders)

	_, err := h.auditpolicy.Create(&policy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeValid, metav1.ConditionFalse, "failed to create filter: failed to compile regex '*': error parsing regexp: missing argument to repetition operator: `*`")

	actual, err := h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)

	_, err = h.OnRemove(testHandlerFuncKey, &policy)
	assert.Error(t, err, "failed to remove policy '%s/%s' from writer", policy.Namespace, policy.Name)

	_, ok = h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)
}

func TestOnRemoveNonexistantPolicy(t *testing.T) {
	policy := samplePolicy

	h := setup(t, auditlogv1.LevelHeaders)

	_, err := h.OnRemove(testHandlerFuncKey, &policy)
	assert.Error(t, err)

	_, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)
}
