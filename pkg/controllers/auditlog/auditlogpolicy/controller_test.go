package auditlogpolicy

import (
	"io"
	"testing"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/controllers/auditlog/auditlogpolicy/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultNamespace   = "default"
	testHandlerFuncKey = "test"
)

var (
	samplePolicy = auditlogv1.AuditLogPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultNamespace,
			Name:      "test",
		},
		Spec: auditlogv1.AuditLogPolicySpec{
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
						".some.path.to.value",
					},
					Keys: []string{
						".*[pP]assword.*",
					},
				},
			},
		},
		Status: auditlogv1.AuditLogPolicyStatus{},
	}
)

func setup(t *testing.T, level auditlogv1.Level) handler {
	writer, err := audit.NewWriter(io.Discard, level)
	if err != nil {
		t.Error("failed to create writer for audit log handler: %w", err)
	}

	h := handler{
		auditlogpolicy: &fake.MockController{
			Objs: map[string]map[string]auditlogv1.AuditLogPolicy{
				defaultNamespace: {},
			},
		},
		writer: writer,
	}

	return h
}

// todo: add assertions to check that policies were added properlly to the writer

// todo: [DONE] test add enabled policy
// todo: [DONE] test add invalid policy (filter uri regex)
// todo: [DONE] test add invalid policy (filter action)
// todo: [DONE] test add invalid policy (redactor header regex)
// todo: [DONE] test add invalid policy (redactor key regex)
// todo: [DONE] test add disabled policy
// todo: [DONE] test disable active policy
// todo: test overwrite active policy with active policy
// todo: [DONE] test overwrite active policy with invalid policy
// todo: [DONE] test overwrite invalid policy with active policy

// todo: [DONE] test remove active policy
// todo: [DONE] test remove disabled policy
// todo: [DONE] test remove invalid policy
// tood: [DONE] test remove nonexistant policy

// todo: will probably need update all tests to replace slices rather then update inplace

func TestOnChangeAddSimplePolicy(t *testing.T) {
	h := setup(t, auditlogv1.LevelMetadata)

	policy := samplePolicy

	_, err := h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	expected.Status = auditlogv1.AuditLogPolicyStatus{
		Condition: auditlogv1.AuditLogPolicyStatusConditionActive,
	}

	actual, err := h.auditlogpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	expectedPolicy, err := audit.PolicyFromAuditLogPolicy(&policy)
	assert.NoError(t, err)

	actualPolicy, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.True(t, ok)
	assert.Equal(t, expectedPolicy, actualPolicy)
}

func TestOnChangeAddInvalidPolicyFilterRequestURIRegex(t *testing.T) {
	h := setup(t, auditlogv1.LevelMetadata)

	policy := samplePolicy
	policy.Spec.Filters = []auditlogv1.Filter{
		{
			Action:     auditlogv1.FilterActionAllow,
			RequestURI: "*",
		},
	}

	_, err := h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	expected.Status = auditlogv1.AuditLogPolicyStatus{
		Condition: auditlogv1.AuditLogPolicyStatusConditionInvalid,
		Message:   "failed to create policy: failed to create filter: failed to compile regex '*': error parsing regexp: missing argument to repetition operator: `*`",
	}

	actual, err := h.auditlogpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)
}

func TestOnChangeAddInvalidPolicyFilterAction(t *testing.T) {
	h := setup(t, auditlogv1.LevelMetadata)

	policy := samplePolicy
	policy.Spec.Filters = []auditlogv1.Filter{
		{
			Action: "don't not allow",
		},
	}

	_, err := h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	expected.Status = auditlogv1.AuditLogPolicyStatus{
		Condition: auditlogv1.AuditLogPolicyStatusConditionInvalid,
		Message:   "failed to create policy: failed to create filter: invalid filter action: 'don't not allow'",
	}

	actual, err := h.auditlogpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)
}

func TestOnChangeAddInvalidPolicyRedactorHeaderRegex(t *testing.T) {
	h := setup(t, auditlogv1.LevelMetadata)

	policy := samplePolicy
	policy.Spec.AdditionalRedactions = []auditlogv1.Redaction{
		{
			Headers: []string{
				"*",
			},
		},
	}

	_, err := h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	expected.Status = auditlogv1.AuditLogPolicyStatus{
		Condition: auditlogv1.AuditLogPolicyStatusConditionInvalid,
		Message:   "failed to create policy: failed to create redactor: failed to compile headers regexes: failed to compile regex: error parsing regexp: missing argument to repetition operator: `*`",
	}

	actual, err := h.auditlogpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)
}

func TestOnChangeAddInvalidPolicyRedactorKeyRegex(t *testing.T) {
	h := setup(t, auditlogv1.LevelMetadata)

	policy := samplePolicy
	policy.Spec.AdditionalRedactions = []auditlogv1.Redaction{
		{
			Keys: []string{
				"*",
			},
		},
	}

	_, err := h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	expected.Status = auditlogv1.AuditLogPolicyStatus{
		Condition: auditlogv1.AuditLogPolicyStatusConditionInvalid,
		Message:   "failed to create policy: failed to create redactor: failed to compile keys regexes: failed to compile regex: error parsing regexp: missing argument to repetition operator: `*`",
	}

	actual, err := h.auditlogpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)
}

func TestOnChangeAddDisablePolicy(t *testing.T) {
	h := setup(t, auditlogv1.LevelMetadata)

	policy := samplePolicy
	policy.Spec.Enabled = false

	_, err := h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	policy.Status = auditlogv1.AuditLogPolicyStatus{
		Condition: auditlogv1.AuditLogPolicyStatusConditionDisabled,
	}

	actual, err := h.auditlogpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)
}

func TestOnChangeDisableActivePolicy(t *testing.T) {
	h := setup(t, auditlogv1.LevelMetadata)

	policy := samplePolicy

	_, err := h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	expected.Status = auditlogv1.AuditLogPolicyStatus{
		Condition: auditlogv1.AuditLogPolicyStatusConditionActive,
	}

	actual, err := h.auditlogpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	policy.Spec.Enabled = false

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected.Spec.Enabled = false
	expected.Status = auditlogv1.AuditLogPolicyStatus{
		Condition: auditlogv1.AuditLogPolicyStatusConditionDisabled,
	}

	actual, err = h.auditlogpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)
}

func TestOnChangeOverwriteActiveWithInvalid(t *testing.T) {
	h := setup(t, auditlogv1.LevelMetadata)

	policy := samplePolicy

	_, err := h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	expected.Status = auditlogv1.AuditLogPolicyStatus{
		Condition: auditlogv1.AuditLogPolicyStatusConditionActive,
	}

	actual, err := h.auditlogpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	expectedPolicy, err := audit.PolicyFromAuditLogPolicy(&policy)
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

	_, err = h.OnChange(testHandlerFuncKey, &invalidPolicy)
	assert.NoError(t, err)

	expected = invalidPolicy
	expected.Status = auditlogv1.AuditLogPolicyStatus{
		Condition: auditlogv1.AuditLogPolicyStatusConditionInvalid,
		Message:   "failed to create policy: failed to create filter: failed to compile regex '*': error parsing regexp: missing argument to repetition operator: `*`",
	}

	actual, err = h.auditlogpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	expectedPolicy, err = audit.PolicyFromAuditLogPolicy(&policy)
	assert.NoError(t, err)

	actualPolicy, ok = h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.True(t, ok)
	assert.Equal(t, expectedPolicy, actualPolicy)
}

func TestOnChangeOverwriteInvalidWithActive(t *testing.T) {
	h := setup(t, auditlogv1.LevelMetadata)

	invalidPolicy := samplePolicy
	invalidPolicy.Spec.Filters = []auditlogv1.Filter{
		{
			Action:     auditlogv1.FilterActionAllow,
			RequestURI: "*",
		},
	}

	_, err := h.OnChange(testHandlerFuncKey, &invalidPolicy)
	assert.NoError(t, err)

	expected := invalidPolicy
	expected.Status = auditlogv1.AuditLogPolicyStatus{
		Condition: auditlogv1.AuditLogPolicyStatusConditionInvalid,
		Message:   "failed to create policy: failed to create filter: failed to compile regex '*': error parsing regexp: missing argument to repetition operator: `*`",
	}

	actual, err := h.auditlogpolicy.Get(invalidPolicy.Namespace, invalidPolicy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, ok := h.writer.GetPolicy(invalidPolicy.Namespace, invalidPolicy.Name)
	assert.False(t, ok)

	policy := samplePolicy

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected = policy
	expected.Status = auditlogv1.AuditLogPolicyStatus{
		Condition: auditlogv1.AuditLogPolicyStatusConditionActive,
	}

	actual, err = h.auditlogpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	expectedPolicy, err := audit.PolicyFromAuditLogPolicy(&policy)
	assert.NoError(t, err)

	actualPolicy, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.True(t, ok)
	assert.Equal(t, expectedPolicy, actualPolicy)
}

func TestOnRemoveActivePolicy(t *testing.T) {
	h := setup(t, auditlogv1.LevelMetadata)

	policy := samplePolicy

	_, err := h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	expected.Status = auditlogv1.AuditLogPolicyStatus{
		Condition: auditlogv1.AuditLogPolicyStatusConditionActive,
	}

	actual, err := h.auditlogpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)

	_, err = h.OnRemove(testHandlerFuncKey, &policy)
	assert.NoError(t, err)
}

func TestOnRemoveDisabledPolicy(t *testing.T) {
	h := setup(t, auditlogv1.LevelMetadata)

	policy := samplePolicy
	policy.Spec.Enabled = false

	_, err := h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	expected.Status = auditlogv1.AuditLogPolicyStatus{
		Condition: auditlogv1.AuditLogPolicyStatusConditionDisabled,
	}

	actual, err := h.auditlogpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
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
	h := setup(t, auditlogv1.LevelMetadata)

	policy := samplePolicy
	policy.Spec.Filters = []auditlogv1.Filter{
		{
			Action:     auditlogv1.FilterActionAllow,
			RequestURI: "*",
		},
	}

	_, err := h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	expected.Status = auditlogv1.AuditLogPolicyStatus{
		Condition: auditlogv1.AuditLogPolicyStatusConditionInvalid,
		Message:   "failed to create policy: failed to create filter: failed to compile regex '*': error parsing regexp: missing argument to repetition operator: `*`",
	}

	actual, err := h.auditlogpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
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
	h := setup(t, auditlogv1.LevelMetadata)

	policy := samplePolicy

	_, err := h.OnRemove(testHandlerFuncKey, &policy)
	assert.Error(t, err)

	_, ok := h.writer.GetPolicy(policy.Namespace, policy.Name)
	assert.False(t, ok)
}
