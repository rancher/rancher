package vai

import (
	"fmt"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/url"
	"sort"
	"strings"

	v1 "k8s.io/api/core/v1"
)

type secretSortTestCase struct {
	name             string
	createSecrets    func(sortFunc func() url.Values) ([]v1.Secret, []string, []string)
	sort             func() url.Values
	supportedWithVai bool
}

func (s secretSortTestCase) SupportedWithVai() bool {
	return s.supportedWithVai
}

// getSortedSecretNames is a helper function to sort secrets based on the provided sort parameters
func getSortedSecretNames(secrets []v1.Secret, sortFunc func() url.Values) []string {
	sortValues := sortFunc()
	sortFields := strings.Split(sortValues.Get("sort"), ",")

	sort.Slice(secrets, func(firstIndex, secondIndex int) bool {
		for _, field := range sortFields {
			ascending := true
			if strings.HasPrefix(field, "-") {
				ascending = false
				field = field[1:]
			}

			var firstValue, secondValue string
			switch field {
			case "metadata.name":
				firstValue, secondValue = secrets[firstIndex].Name, secrets[secondIndex].Name
			case "metadata.namespace":
				firstValue, secondValue = secrets[firstIndex].Namespace, secrets[secondIndex].Namespace
			default:
				continue
			}

			if firstValue != secondValue {
				if ascending {
					return firstValue < secondValue
				}
				return firstValue > secondValue
			}
		}
		return false
	})

	sortedNames := make([]string, len(secrets))
	for index, secret := range secrets {
		sortedNames[index] = secret.Name
	}
	return sortedNames
}

var secretSortTestCases = []secretSortTestCase{
	{
		name: "Sort by name ascending",
		createSecrets: func(sortFunc func() url.Values) ([]v1.Secret, []string, []string) {
			suffix := namegen.RandStringLower(randomStringLength)
			ns1 := fmt.Sprintf("namespace1-%s", suffix)
			ns2 := fmt.Sprintf("namespace2-%s", suffix)
			secrets := []v1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: ns1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: ns1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret3", Namespace: ns2}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret4", Namespace: ns2}},
			}
			return secrets, getSortedSecretNames(secrets, sortFunc), []string{ns1, ns2}
		},
		sort: func() url.Values {
			return url.Values{"sort": []string{"metadata.name"}}
		},
		supportedWithVai: true,
	},
	{
		name: "Sort by name descending",
		createSecrets: func(sortFunc func() url.Values) ([]v1.Secret, []string, []string) {
			suffix := namegen.RandStringLower(randomStringLength)
			ns1 := fmt.Sprintf("namespace1-%s", suffix)
			ns2 := fmt.Sprintf("namespace2-%s", suffix)
			secrets := []v1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: ns1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: ns1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret3", Namespace: ns2}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret4", Namespace: ns2}},
			}
			return secrets, getSortedSecretNames(secrets, sortFunc), []string{ns1, ns2}
		},
		sort: func() url.Values {
			return url.Values{"sort": []string{"-metadata.name"}}
		},
		supportedWithVai: true,
	},
	{
		name: "Sort by namespace ascending",
		createSecrets: func(sortFunc func() url.Values) ([]v1.Secret, []string, []string) {
			suffix := namegen.RandStringLower(randomStringLength)
			ns1 := fmt.Sprintf("namespace1-%s", suffix)
			ns2 := fmt.Sprintf("namespace2-%s", suffix)
			secrets := []v1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: ns1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: ns1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret3", Namespace: ns2}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret4", Namespace: ns2}},
			}
			return secrets, getSortedSecretNames(secrets, sortFunc), []string{ns1, ns2}
		},
		sort: func() url.Values {
			return url.Values{"sort": []string{"metadata.namespace"}}
		},
		supportedWithVai: true,
	},
	{
		name: "Sort by namespace descending",
		createSecrets: func(sortFunc func() url.Values) ([]v1.Secret, []string, []string) {
			suffix := namegen.RandStringLower(randomStringLength)
			ns1 := fmt.Sprintf("namespace1-%s", suffix)
			ns2 := fmt.Sprintf("namespace2-%s", suffix)
			secrets := []v1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: ns1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: ns1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret3", Namespace: ns2}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret4", Namespace: ns2}},
			}
			return secrets, getSortedSecretNames(secrets, sortFunc), []string{ns1, ns2}
		},
		sort: func() url.Values {
			return url.Values{"sort": []string{"-metadata.namespace,metadata.name"}}
		},
		supportedWithVai: true,
	},
	{
		name: "Sort by name and namespace ascending",
		createSecrets: func(sortFunc func() url.Values) ([]v1.Secret, []string, []string) {
			suffix := namegen.RandStringLower(randomStringLength)
			ns1 := fmt.Sprintf("namespace1-%s", suffix)
			ns2 := fmt.Sprintf("namespace2-%s", suffix)
			secrets := []v1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: ns1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: ns1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret3", Namespace: ns2}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret4", Namespace: ns2}},
			}
			return secrets, getSortedSecretNames(secrets, sortFunc), []string{ns1, ns2}
		},
		sort: func() url.Values {
			return url.Values{"sort": []string{"metadata.name,metadata.namespace"}}
		},
		supportedWithVai: true,
	},
	{
		name: "Sort by namespace and name ascending",
		createSecrets: func(sortFunc func() url.Values) ([]v1.Secret, []string, []string) {
			suffix := namegen.RandStringLower(randomStringLength)
			ns1 := fmt.Sprintf("namespace1-%s", suffix)
			ns2 := fmt.Sprintf("namespace2-%s", suffix)
			secrets := []v1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: ns1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: ns1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret3", Namespace: ns2}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret4", Namespace: ns2}},
			}
			return secrets, getSortedSecretNames(secrets, sortFunc), []string{ns1, ns2}
		},
		sort: func() url.Values {
			return url.Values{"sort": []string{"metadata.namespace,metadata.name"}}
		},
		supportedWithVai: true,
	},
	{
		name: "Sort by name ascending and namespace descending",
		createSecrets: func(sortFunc func() url.Values) ([]v1.Secret, []string, []string) {
			suffix := namegen.RandStringLower(randomStringLength)
			ns1 := fmt.Sprintf("namespace1-%s", suffix)
			ns2 := fmt.Sprintf("namespace2-%s", suffix)
			secrets := []v1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: ns1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: ns1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret3", Namespace: ns2}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret4", Namespace: ns2}},
			}
			return secrets, getSortedSecretNames(secrets, sortFunc), []string{ns1, ns2}
		},
		sort: func() url.Values {
			return url.Values{"sort": []string{"metadata.name,-metadata.namespace"}}
		},
		supportedWithVai: true,
	},
	{
		name: "Sort by namespace descending and name ascending",
		createSecrets: func(sortFunc func() url.Values) ([]v1.Secret, []string, []string) {
			suffix := namegen.RandStringLower(randomStringLength)
			ns1 := fmt.Sprintf("namespace1-%s", suffix)
			ns2 := fmt.Sprintf("namespace2-%s", suffix)
			secrets := []v1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: ns1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: ns1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret3", Namespace: ns2}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret4", Namespace: ns2}},
			}
			return secrets, getSortedSecretNames(secrets, sortFunc), []string{ns1, ns2}
		},
		sort: func() url.Values {
			return url.Values{"sort": []string{"-metadata.namespace,metadata.name"}}
		},
		supportedWithVai: true,
	},
}
