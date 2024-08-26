package vai

import (
	"fmt"
	"net/url"

	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type secretFilterTestCase struct {
	name             string
	createSecrets    func() ([]v1.Secret, []string, []string, []string)
	filter           func(namespaces []string) url.Values
	supportedWithVai bool
}

func (s secretFilterTestCase) SupportedWithVai() bool {
	return s.supportedWithVai
}

var secretFilterTestCases = []secretFilterTestCase{
	{
		name: "Filter with negation and namespace",
		createSecrets: func() ([]v1.Secret, []string, []string, []string) {
			suffix := namegen.RandStringLower(randomStringLength)
			ns := fmt.Sprintf("namespace-%s", suffix)

			name1 := fmt.Sprintf("secret1-%s", namegen.RandStringLower(randomStringLength))
			name2 := fmt.Sprintf("secret2-%s", namegen.RandStringLower(randomStringLength))
			name3 := fmt.Sprintf("config3-%s", namegen.RandStringLower(randomStringLength))

			secrets := []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{Name: name1, Namespace: ns},
					Type:       v1.SecretTypeOpaque,
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: name2, Namespace: ns},
					Type:       v1.SecretTypeOpaque,
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: name3, Namespace: ns},
					Type:       v1.SecretTypeOpaque,
				},
			}

			expectedNames := []string{name1, name2}
			allNamespaces := []string{ns}
			expectedNamespaces := []string{ns}

			return secrets, expectedNames, allNamespaces, expectedNamespaces
		},
		filter: func(namespaces []string) url.Values {
			return url.Values{
				"filter":               []string{"metadata.name!=config"},
				"projectsornamespaces": namespaces,
			}
		},
		supportedWithVai: true,
	},
	{
		name: "Filter by namespace",
		createSecrets: func() ([]v1.Secret, []string, []string, []string) {
			suffix := namegen.RandStringLower(randomStringLength)
			ns1 := fmt.Sprintf("namespace1-%s", suffix)
			ns2 := fmt.Sprintf("namespace2-%s", suffix)
			name1 := fmt.Sprintf("secret1-%s", namegen.RandStringLower(randomStringLength))
			name2 := fmt.Sprintf("secret2-%s", namegen.RandStringLower(randomStringLength))

			secrets := []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{Name: name1, Namespace: ns1},
					Type:       v1.SecretTypeOpaque,
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: name2, Namespace: ns2},
					Type:       v1.SecretTypeOpaque,
				},
			}

			expectedNames := []string{name1}
			allNamespaces := []string{ns1, ns2}
			expectedNamespaces := []string{ns1}

			return secrets, expectedNames, allNamespaces, expectedNamespaces
		},
		filter: func(namespaces []string) url.Values {
			return url.Values{
				"projectsornamespaces": []string{namespaces[0]},
			}
		},
		supportedWithVai: true,
	},
	{
		name: "Filter by name and namespace",
		createSecrets: func() ([]v1.Secret, []string, []string, []string) {
			suffix := namegen.RandStringLower(randomStringLength)
			ns := fmt.Sprintf("namespace-%s", suffix)
			name1 := fmt.Sprintf("secret1-%s", namegen.RandStringLower(randomStringLength))
			name2 := fmt.Sprintf("config2-%s", namegen.RandStringLower(randomStringLength))

			secrets := []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{Name: name1, Namespace: ns},
					Type:       v1.SecretTypeOpaque,
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: name2, Namespace: ns},
					Type:       v1.SecretTypeOpaque,
				},
			}

			expectedNames := []string{name1}
			allNamespaces := []string{ns}
			expectedNamespaces := []string{ns}

			return secrets, expectedNames, allNamespaces, expectedNamespaces
		},
		filter: func(namespaces []string) url.Values {
			return url.Values{
				"filter":               []string{"metadata.name=secret1"},
				"projectsornamespaces": namespaces,
			}
		},
		supportedWithVai: true,
	},
	{
		name: "Filter by single label",
		createSecrets: func() ([]v1.Secret, []string, []string, []string) {
			suffix := namegen.RandStringLower(randomStringLength)
			ns := fmt.Sprintf("namespace-%s", suffix)
			name1 := fmt.Sprintf("secret1-%s", namegen.RandStringLower(randomStringLength))
			name2 := fmt.Sprintf("secret2-%s", namegen.RandStringLower(randomStringLength))

			secrets := []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name1,
						Namespace: ns,
						Labels:    map[string]string{"key1": "value1"},
					},
					Type: v1.SecretTypeOpaque,
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name2,
						Namespace: ns,
						Labels:    map[string]string{"key1": "value2"},
					},
					Type: v1.SecretTypeOpaque,
				},
			}

			expectedNames := []string{name1}
			allNamespaces := []string{ns}
			expectedNamespaces := []string{ns}

			return secrets, expectedNames, allNamespaces, expectedNamespaces
		},
		filter: func(namespaces []string) url.Values {
			return url.Values{
				"filter":               []string{"metadata.labels.key1=value1"},
				"projectsornamespaces": namespaces,
			}
		},
		supportedWithVai: false,
	},
	{
		name: "Filter by multiple labels",
		createSecrets: func() ([]v1.Secret, []string, []string, []string) {
			suffix := namegen.RandStringLower(randomStringLength)
			ns := fmt.Sprintf("namespace-%s", suffix)
			name1 := fmt.Sprintf("secret1-%s", namegen.RandStringLower(randomStringLength))
			name2 := fmt.Sprintf("secret2-%s", namegen.RandStringLower(randomStringLength))
			name3 := fmt.Sprintf("secret3-%s", namegen.RandStringLower(randomStringLength))

			secrets := []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name1,
						Namespace: ns,
						Labels:    map[string]string{"key1": "value1", "key2": "value2"},
					},
					Type: v1.SecretTypeOpaque,
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name2,
						Namespace: ns,
						Labels:    map[string]string{"key1": "value1"},
					},
					Type: v1.SecretTypeOpaque,
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name3,
						Namespace: ns,
						Labels:    map[string]string{"key2": "value2"},
					},
					Type: v1.SecretTypeOpaque,
				},
			}

			expectedNames := []string{name1}
			allNamespaces := []string{ns}
			expectedNamespaces := []string{ns}

			return secrets, expectedNames, allNamespaces, expectedNamespaces
		},
		filter: func(namespaces []string) url.Values {
			return url.Values{
				"filter":               []string{"metadata.labels.key1=value1", "metadata.labels.key2=value2"},
				"projectsornamespaces": namespaces,
			}
		},
		supportedWithVai: false,
	},
}
