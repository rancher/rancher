package auditpolicy

import (
	"io"
	"testing"
	"time"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/controllers/auditlog/auditpolicy/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func mockTimeFactory() metav1.Time {
	return metav1.Time{
		Time: time.Time{},
	}
}

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

		time: mockTimeFactory,
	}

	return h
}

func inject(t *testing.T, p auditlogv1.AuditPolicy, name string) auditlogv1.AuditPolicy {
	t.Helper()

	p.Namespace = TargetNamespace
	p.Name = name

	return p
}

func TestOnChange(t *testing.T) {
	type testCase struct {
		name     string
		input    auditlogv1.AuditPolicy
		expected auditlogv1.AuditPolicyStatus
	}

	cases := []testCase{
		{
			name: "AddSimplePolicy",
			input: auditlogv1.AuditPolicy{
				Spec: auditlogv1.AuditPolicySpec{
					Enabled: true,
				},
			},
			expected: auditlogv1.AuditPolicyStatus{
				Conditions: []metav1.Condition{
					metav1.Condition{
						Type:               string(auditlogv1.AuditPolicyConditionTypeActive),
						Status:             metav1.ConditionTrue,
						LastTransitionTime: mockTimeFactory(),
					},
					metav1.Condition{
						Type:               string(auditlogv1.AuditPolicyConditionTypeValid),
						Status:             metav1.ConditionTrue,
						LastTransitionTime: mockTimeFactory(),
					},
				},
			},
		},

		{
			name: "AddInvalidPolicyFilterRequestURIRegex",
			input: auditlogv1.AuditPolicy{
				Spec: auditlogv1.AuditPolicySpec{
					Enabled: true,
					Filters: []auditlogv1.Filter{
						{
							Action:     auditlogv1.FilterActionAllow,
							RequestURI: "*",
						},
					},
				},
			},
			expected: auditlogv1.AuditPolicyStatus{
				Conditions: []metav1.Condition{
					metav1.Condition{
						Type:               string(auditlogv1.AuditPolicyConditionTypeActive),
						Status:             metav1.ConditionUnknown,
						LastTransitionTime: mockTimeFactory(),
					},
					metav1.Condition{
						Type:               string(auditlogv1.AuditPolicyConditionTypeValid),
						Status:             metav1.ConditionFalse,
						LastTransitionTime: mockTimeFactory(),
						Message:            "failed to create filter: failed to compile regex '*': error parsing regexp: missing argument to repetition operator: `*`",
					},
				},
			},
		},

		{
			name: "AddInvalidPolicyFilterAction",
			input: auditlogv1.AuditPolicy{
				Spec: auditlogv1.AuditPolicySpec{
					Enabled: true,
					Filters: []auditlogv1.Filter{
						{
							Action: "do not allow",
						},
					},
				},
			},
			expected: auditlogv1.AuditPolicyStatus{
				Conditions: []metav1.Condition{
					metav1.Condition{
						Type:               string(auditlogv1.AuditPolicyConditionTypeActive),
						Status:             metav1.ConditionUnknown,
						LastTransitionTime: mockTimeFactory(),
					},
					metav1.Condition{
						Type:               string(auditlogv1.AuditPolicyConditionTypeValid),
						Status:             metav1.ConditionFalse,
						LastTransitionTime: mockTimeFactory(),
						Message:            "failed to create filter: invalid filter action: 'do not allow'",
					},
				},
			},
		},

		{
			name: "AddInvalidPolicyRedactorHeaderRegex",
			input: auditlogv1.AuditPolicy{
				Spec: auditlogv1.AuditPolicySpec{
					Enabled: true,
					AdditionalRedactions: []auditlogv1.Redaction{
						{
							Headers: []string{
								"*",
							},
						},
					},
				},
			},
			expected: auditlogv1.AuditPolicyStatus{
				Conditions: []metav1.Condition{
					metav1.Condition{
						Type:               string(auditlogv1.AuditPolicyConditionTypeActive),
						Status:             metav1.ConditionUnknown,
						LastTransitionTime: mockTimeFactory(),
					},
					metav1.Condition{
						Type:               string(auditlogv1.AuditPolicyConditionTypeValid),
						Status:             metav1.ConditionFalse,
						LastTransitionTime: mockTimeFactory(),
						Message:            "failed to create redactor: failed to compile headers regexes: failed to compile regex: error parsing regexp: missing argument to repetition operator: `*`",
					},
				},
			},
		},

		{
			name: "AddInvalidPolicyRedactorJSONPath",
			input: auditlogv1.AuditPolicy{
				Spec: auditlogv1.AuditPolicySpec{
					Enabled: true,
					AdditionalRedactions: []auditlogv1.Redaction{
						{
							Paths: []string{
								".missing.root",
							},
						},
					},
				},
			},
			expected: auditlogv1.AuditPolicyStatus{
				Conditions: []metav1.Condition{
					metav1.Condition{
						Type:               string(auditlogv1.AuditPolicyConditionTypeActive),
						Status:             metav1.ConditionUnknown,
						LastTransitionTime: mockTimeFactory(),
					},
					metav1.Condition{
						Type:               string(auditlogv1.AuditPolicyConditionTypeValid),
						Status:             metav1.ConditionFalse,
						LastTransitionTime: mockTimeFactory(),
						Message:            "failed to create redactor: failed to parse paths: failed to parse jsonpath: paths must begin with the root object identifier: '$'",
					},
				},
			},
		},

		{
			name: "AddDisabledPolicy",
			input: auditlogv1.AuditPolicy{
				Spec: auditlogv1.AuditPolicySpec{},
			},
			expected: auditlogv1.AuditPolicyStatus{
				Conditions: []metav1.Condition{
					metav1.Condition{
						Type:               string(auditlogv1.AuditPolicyConditionTypeActive),
						Status:             metav1.ConditionFalse,
						LastTransitionTime: mockTimeFactory(),
						Message:            "policy was disabled",
					},
					metav1.Condition{
						Type:               string(auditlogv1.AuditPolicyConditionTypeValid),
						Status:             metav1.ConditionUnknown,
						LastTransitionTime: mockTimeFactory(),
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := setup(t, auditlogv1.LevelHeaders)
			c.input = inject(t, c.input, c.name)

			_, err := h.auditpolicy.Create(&c.input)
			require.NoError(t, err)

			actual, err := h.OnChange(testHandlerFuncKey, &c.input)
			assert.NoError(t, err)

			assert.Equal(t, c.expected, actual.Status)
		})
	}
}

func TestOnChangeDisableActivePolicy(t *testing.T) {
	policy := samplePolicy

	h := setup(t, auditlogv1.LevelHeaders)

	_, err := h.auditpolicy.Create(&policy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected := policy
	h.updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionTrue, "")

	actual, err := h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expected, *actual)

	policy.Spec.Enabled = false

	_, err = h.auditpolicy.Update(&policy)
	assert.NoError(t, err)

	_, err = h.OnChange(testHandlerFuncKey, &policy)
	assert.NoError(t, err)

	expected.Spec.Enabled = false
	h.updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionFalse, "")

	actual, err = h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expected, *actual)

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
	// updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionTrue, "")

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
	// updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeValid, metav1.ConditionFalse, "failed to create filter: failed to compile regex '*': error parsing regexp: missing argument to repetition operator: `*`")

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
	// updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeValid, metav1.ConditionFalse, "failed to create filter: failed to compile regex '*': error parsing regexp: missing argument to repetition operator: `*`")

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
	// updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionFalse, "")

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
	h.updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionTrue, "")

	actual, err := h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expected, *actual)

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
	h.updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionFalse, "")

	actual, err := h.auditpolicy.Get(policy.Namespace, policy.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expected, *actual)

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
	h.updateStatus(&expected, auditlogv1.AuditPolicyConditionTypeValid, metav1.ConditionFalse, "failed to create filter: failed to compile regex '*': error parsing regexp: missing argument to repetition operator: `*`")

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
