//go:build (validation || infra.any || cluster.any || extended) && !stress

package vai

import (
	"fmt"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/kubectl"
	"github.com/rancher/shepherd/extensions/vai"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

const scriptURL = "https://raw.githubusercontent.com/rancher/rancher/main/tests/v2/validation/steve/vai/scripts/script.sh"

type VaiTestSuite struct {
	suite.Suite
	client      *rancher.Client
	steveClient *steveV1.Client
	session     *session.Session
	cluster     management.Cluster
	vaiEnabled  bool
	once        sync.Once
}

func (v *VaiTestSuite) SetupSuite() {
	testSession := session.NewSession()
	v.session = testSession

	client, err := rancher.NewClient("", v.session)
	require.NoError(v.T(), err)

	v.client = client
	v.steveClient = client.Steve

	enabled, err := isVaiEnabled(v.client)

	require.NoError(v.T(), err)
	v.vaiEnabled = enabled
}

func (v *VaiTestSuite) TearDownSuite() {
	v.session.Cleanup()
}

func (v *VaiTestSuite) enableVai() error {
	logrus.Info("Enabling VAI caching")
	startTime := time.Now()
	err := enableVAI(v.client, &v.vaiEnabled, &v.once)
	if err != nil {
		return err
	}
	duration := time.Since(startTime)
	logrus.Infof("Enabling VAI took %s", formatDuration(duration))
	v.vaiEnabled = true
	return nil
}

func (v *VaiTestSuite) disableVai() error {
	logrus.Info("Disabling VAI caching")
	startTime := time.Now()
	err := vai.DisableVaiCaching(v.client)
	if err != nil {
		return err
	}
	duration := time.Since(startTime)
	logrus.Infof("Disabling VAI took %s", formatDuration(duration))
	v.vaiEnabled = false
	return nil
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%d hours %d minutes %d seconds", h, m, s)
	} else if m > 0 {
		return fmt.Sprintf("%d minutes %d seconds", m, s)
	} else {
		return fmt.Sprintf("%d seconds", s)
	}
}

func (v *VaiTestSuite) TestVAI() {
	v.Run("InitialState", func() {
		if v.vaiEnabled {
			v.Run("TestWithVaiInitiallyEnabled", v.testWithVaiEnabled)
		} else {
			v.Run("TestWithVaiInitiallyDisabled", v.testWithVaiDisabled)
		}
	})

	v.Run("ToggleVaiAndRetest", func() {
		initialState := v.vaiEnabled
		if initialState {
			err := v.disableVai()
			require.NoError(v.T(), err)
			v.testWithVaiDisabled()
		} else {
			err := v.enableVai()
			require.NoError(v.T(), err)
			v.testWithVaiEnabled()
		}
	})
}

func (v *VaiTestSuite) testWithVaiEnabled() {
	v.Run("SecretFilters", func() {
		supportedWithVai := filterTestCases(secretFilterTestCases, v.vaiEnabled)
		v.runSecretFilterTestCases(supportedWithVai)
	})

	v.Run("PodFilters", func() {
		supportedWithVai := filterTestCases(podFilterTestCases, v.vaiEnabled)
		v.runPodFilterTestCases(supportedWithVai)
	})

	v.Run("SecretSorting", func() {
		supportedWithVai := filterTestCases(secretSortTestCases, v.vaiEnabled)
		v.runSecretSortTestCases(supportedWithVai)
	})

	v.Run("SecretLimit", func() {
		supportedWithVai := filterTestCases(secretLimitTestCases, v.vaiEnabled)
		v.runSecretLimitTestCases(supportedWithVai)
	})

	v.Run("CheckDBFilesInPods", v.checkDBFilesInPods)
	v.Run("CheckSecretInDB", v.checkSecretInVAIDatabase)
	v.Run("CheckNamespaceInAllVAIDatabases", v.checkNamespaceInAllVAIDatabases)
}

func (v *VaiTestSuite) testWithVaiDisabled() {
	v.Run("SecretFilters", func() {
		unsupportedWithVai := filterTestCases(secretFilterTestCases, v.vaiEnabled)
		v.runSecretFilterTestCases(unsupportedWithVai)
	})

	v.Run("PodFilters", func() {
		unsupportedWithVai := filterTestCases(podFilterTestCases, v.vaiEnabled)
		v.runPodFilterTestCases(unsupportedWithVai)
	})

	v.Run("SecretSorting", func() {
		unsupportedWithVai := filterTestCases(secretSortTestCases, v.vaiEnabled)
		v.runSecretSortTestCases(unsupportedWithVai)
	})

	v.Run("SecretLimit", func() {
		unsupportedWithVai := filterTestCases(secretLimitTestCases, v.vaiEnabled)
		v.runSecretLimitTestCases(unsupportedWithVai)
	})

	v.Run("NormalOperations", v.testNormalOperationsWithVaiDisabled)
}

func (v *VaiTestSuite) testNormalOperationsWithVaiDisabled() {
	pods, err := v.client.Steve.SteveType("pod").List(nil)
	require.NoError(v.T(), err)
	assert.NotEmpty(v.T(), pods.Data, "Should be able to list pods even with VAI disabled")
}

func (v *VaiTestSuite) runSecretFilterTestCases(testCases []secretFilterTestCase) {
	secretClient := v.steveClient.SteveType("secret")
	namespaceClient := v.steveClient.SteveType("namespace")

	for _, tc := range testCases {
		v.Run(tc.name, func() {
			logrus.Infof("Starting case: %s", tc.name)
			logrus.Infof("Running with vai enabled: [%v]", v.vaiEnabled)

			secrets, expectedNames, allNamespaces, expectedNamespaces := tc.createSecrets()

			for _, ns := range allNamespaces {
				logrus.Infof("Creating namespace: %s", ns)
				_, err := namespaceClient.Create(&coreV1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: ns,
					},
				})
				require.NoError(v.T(), err)
			}

			createdSecrets := make([]steveV1.SteveAPIObject, len(secrets))
			for i, secret := range secrets {
				created, err := secretClient.Create(&secret)
				require.NoError(v.T(), err)
				createdSecrets[i] = *created
			}

			filterValues := tc.filter(expectedNamespaces)

			secretCollection, err := secretClient.List(filterValues)
			require.NoError(v.T(), err)

			var actualNames []string
			for _, item := range secretCollection.Data {
				actualNames = append(actualNames, item.GetName())
			}

			require.Equal(v.T(), len(expectedNames), len(actualNames), "Number of returned secrets doesn't match expected")
			for _, expectedName := range expectedNames {
				require.Contains(v.T(), actualNames, expectedName, fmt.Sprintf("Expected secret %s not found in actual secrets", expectedName))
			}
		})
	}
}

func (v *VaiTestSuite) runPodFilterTestCases(testCases []podFilterTestCase) {
	podClient := v.steveClient.SteveType("pod")
	namespaceClient := v.steveClient.SteveType("namespace")

	for _, tc := range testCases {
		v.Run(tc.name, func() {
			logrus.Infof("Starting case: %s", tc.name)
			logrus.Infof("Running with vai enabled: [%v]", v.vaiEnabled)

			pods, expectedNames, allNamespaces, expectedNamespaces := tc.createPods()

			for _, ns := range allNamespaces {
				logrus.Infof("Creating namespace: %s", ns)
				_, err := namespaceClient.Create(&coreV1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: ns,
					},
				})
				require.NoError(v.T(), err)
			}

			createdPods := make([]steveV1.SteveAPIObject, len(pods))
			for i, pod := range pods {
				created, err := podClient.Create(&pod)
				require.NoError(v.T(), err)
				createdPods[i] = *created
			}

			filterValues := tc.filter(expectedNamespaces)

			podCollection, err := podClient.List(filterValues)
			require.NoError(v.T(), err)

			var actualNames []string
			for _, item := range podCollection.Data {
				actualNames = append(actualNames, item.GetName())
			}

			if tc.expectFound {
				require.Equal(v.T(), len(expectedNames), len(actualNames), "Number of returned pods doesn't match expected")
				for _, expectedName := range expectedNames {
					require.Contains(v.T(), actualNames, expectedName, fmt.Sprintf("Expected pod %s not found in actual pods", expectedName))
				}
			} else {
				require.Empty(v.T(), actualNames, "Expected no pods to be found, but some were returned")
			}
		})
	}
}

func (v *VaiTestSuite) runSecretSortTestCases(testCases []secretSortTestCase) {
	secretClient := v.steveClient.SteveType("secret")
	namespaceClient := v.steveClient.SteveType("namespace")

	for _, tc := range testCases {
		v.Run(tc.name, func() {
			logrus.Infof("Starting case: %s", tc.name)
			logrus.Infof("Running with vai enabled: [%v]", v.vaiEnabled)

			secrets, sortedNames, namespaces := tc.createSecrets(tc.sort)

			for _, ns := range namespaces {
				_, err := namespaceClient.Create(&coreV1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: ns,
					},
				})
				require.NoError(v.T(), err)
			}

			for _, secret := range secrets {
				_, err := secretClient.Create(&secret)
				require.NoError(v.T(), err)
			}

			sortValues := tc.sort()
			sortValues.Add("projectsornamespaces", strings.Join(namespaces, ","))

			secretCollection, err := secretClient.List(sortValues)
			require.NoError(v.T(), err)

			var actualNames []string
			for _, item := range secretCollection.Data {
				actualNames = append(actualNames, item.GetName())
			}

			require.Equal(v.T(), len(sortedNames), len(actualNames), "Number of returned secrets doesn't match expected")
			for i, expectedName := range sortedNames {
				require.Equal(v.T(), expectedName, actualNames[i], fmt.Sprintf("Secret at position %d doesn't match expected order", i))
			}
		})
	}
}

func (v *VaiTestSuite) runSecretLimitTestCases(testCases []secretLimitTestCase) {
	for _, tc := range testCases {
		v.Run(tc.name, func() {
			logrus.Infof("Starting case: %s", tc.name)
			logrus.Infof("Running with vai enabled: [%v]", v.vaiEnabled)

			secrets, ns := tc.createSecrets()

			namespaceClient := v.steveClient.SteveType("namespace")
			_, err := namespaceClient.Create(&coreV1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			})
			require.NoError(v.T(), err)

			secretClient := v.steveClient.SteveType("secret").NamespacedSteveClient(ns)
			for _, secret := range secrets {
				_, err := secretClient.Create(&secret)
				require.NoError(v.T(), err)
			}

			var retrievedSecrets []coreV1.Secret
			var continueToken string
			for {
				params := url.Values{}
				params.Set("limit", fmt.Sprintf("%d", tc.limit))
				if continueToken != "" {
					params.Set("continue", continueToken)
				}

				secretCollection, err := secretClient.List(params)
				require.NoError(v.T(), err)

				for _, obj := range secretCollection.Data {
					var secret coreV1.Secret
					err := steveV1.ConvertToK8sType(obj.JSONResp, &secret)
					require.NoError(v.T(), err)
					retrievedSecrets = append(retrievedSecrets, secret)
				}

				if secretCollection.Pagination == nil || secretCollection.Pagination.Next == "" {
					break
				}
				nextURL, err := url.Parse(secretCollection.Pagination.Next)
				require.NoError(v.T(), err)
				continueToken = nextURL.Query().Get("continue")
			}

			require.Equal(v.T(), tc.expectedTotal, len(retrievedSecrets), "Number of retrieved secrets doesn't match expected")

			expectedSecrets := make(map[string]bool)
			for _, secret := range secrets {
				expectedSecrets[secret.Name] = false
			}

			for _, secret := range retrievedSecrets {
				_, ok := expectedSecrets[secret.Name]
				require.True(v.T(), ok, "Unexpected secret: %s", secret.Name)
				expectedSecrets[secret.Name] = true
			}

			for name, found := range expectedSecrets {
				require.True(v.T(), found, "Expected secret not found: %s", name)
			}
		})
	}
}

func (v *VaiTestSuite) checkDBFilesInPods() {
	expectedDBFiles := []string{"informer_object_cache.db", "informer_object_fields.db"}

	rancherPods, err := listRancherPods(v.client)
	require.NoError(v.T(), err)

	for _, pod := range rancherPods {
		v.T().Run(fmt.Sprintf("Checking pod %s", pod), func(t *testing.T) {
			lsCmd := []string{"kubectl", "exec", pod, "-n", "cattle-system", "--", "ls"}
			output, err := kubectl.Command(v.client, nil, "local", lsCmd, "")
			if err != nil {
				t.Errorf("Error executing command in pod %s: %v", pod, err)
				return
			}

			files := strings.Fields(output)
			var dbFiles []string
			for _, file := range files {
				if strings.HasSuffix(file, ".db") {
					dbFiles = append(dbFiles, file)
				}
			}

			for _, expectedFile := range expectedDBFiles {
				found := false
				for _, dbFile := range dbFiles {
					if dbFile == expectedFile {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected file %s not found in pod %s", expectedFile, pod)
				}
			}
		})
	}
}

func (v *VaiTestSuite) checkSecretInVAIDatabase() {
	v.T().Log("Starting checkSecretInVAIDatabase test")

	secretName := fmt.Sprintf("db-secret-%s", namegen.RandStringLower(randomStringLength))
	v.T().Logf("Generated secret name: %s", secretName)

	secret := &coreV1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: "default"},
		Type:       coreV1.SecretTypeOpaque,
	}

	secretClient := v.steveClient.SteveType("secret")
	v.T().Log("Creating secret...")
	_, err := secretClient.Create(secret)
	require.NoError(v.T(), err)
	v.T().Log("Secret created successfully")

	v.T().Log("Listing Rancher pods...")
	rancherPods, err := listRancherPods(v.client)
	require.NoError(v.T(), err)
	v.T().Logf("Found %d Rancher pods", len(rancherPods))

	v.T().Logf("Using script URL: %s", scriptURL)

	secretFound := false
	var outputs []string

	v.T().Log("List all secrets to hydrate database...")
	_, err = v.client.Steve.SteveType("secret").List(nil)
	require.NoError(v.T(), err)

	for i, pod := range rancherPods {
		v.T().Logf("Processing pod %d: %s", i+1, pod)
		cmd := []string{
			"kubectl", "exec", pod, "-n", "cattle-system", "--",
			"sh", "-c",
			fmt.Sprintf("curl -k -sSL %s | TABLE_NAME='_v1_Secret_fields' RESOURCE_NAME='%s' sh", scriptURL, secretName),
		}

		v.T().Logf("Executing command on pod %s", pod)
		output, err := kubectl.Command(v.client, nil, "local", cmd, "")
		if err != nil {
			v.T().Logf("Error executing script on pod %s: %v", pod, err)
			continue
		}
		v.T().Logf("Command executed successfully on pod %s", pod)

		outputs = append(outputs, fmt.Sprintf("Output from pod %s:\n%s", pod, output))

		if strings.Contains(output, secretName) {
			v.T().Logf("Secret found in pod %s", pod)
			secretFound = true
			break
		} else {
			v.T().Logf("Secret not found in pod %s", pod)
		}
	}

	v.T().Log("Logging all outputs:")
	for i, output := range outputs {
		v.T().Logf("Output %d:\n%s", i+1, output)
	}

	v.T().Logf("Secret found status: %v", secretFound)
	assert.True(v.T(), secretFound, fmt.Sprintf("Secret %s not found in any of the Rancher pods' databases", secretName))
	v.T().Log("checkSecretInVAIDatabase test completed")
}

func (v *VaiTestSuite) checkNamespaceInAllVAIDatabases() {
	v.T().Log("Starting TestCheckNamespaceInAllVAIDatabases test")

	namespaceName := fmt.Sprintf("db-namespace-%s", namegen.RandStringLower(randomStringLength))
	v.T().Logf("Generated namespace name: %s", namespaceName)

	namespace := &coreV1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
	}

	namespaceClient := v.steveClient.SteveType("namespace")
	v.T().Log("Creating namespace...")
	_, err := namespaceClient.Create(namespace)
	require.NoError(v.T(), err)
	v.T().Log("Namespace created successfully")

	_, err = v.client.Steve.SteveType("namespace").List(nil)
	require.NoError(v.T(), err)

	v.T().Log("Listing Rancher pods...")
	rancherPods, err := listRancherPods(v.client)
	require.NoError(v.T(), err)
	v.T().Logf("Found %d Rancher pods", len(rancherPods))

	v.T().Logf("Using script URL: %s", scriptURL)

	var outputs []string
	namespaceFoundCount := 0

	for i, pod := range rancherPods {
		v.T().Logf("Processing pod %d: %s", i+1, pod)
		cmd := []string{
			"kubectl", "exec", pod, "-n", "cattle-system", "--",
			"sh", "-c",
			fmt.Sprintf("curl -k -sSL %s | TABLE_NAME='_v1_Namespace_fields' RESOURCE_NAME='%s' sh", scriptURL, namespaceName),
		}

		v.T().Logf("Executing command on pod %s", pod)
		output, err := kubectl.Command(v.client, nil, "local", cmd, "")
		if err != nil {
			v.T().Logf("Error executing script on pod %s: %v", pod, err)
			continue
		}
		v.T().Logf("Command executed successfully on pod %s", pod)

		outputs = append(outputs, fmt.Sprintf("Output from pod %s:\n%s", pod, output))

		if strings.Contains(output, namespaceName) {
			v.T().Logf("Namespace found in pod %s", pod)
			namespaceFoundCount++
		} else {
			v.T().Logf("Namespace not found in pod %s", pod)
		}
	}

	v.T().Log("Logging all outputs:")
	for i, output := range outputs {
		v.T().Logf("Output %d:\n%s", i+1, output)
	}

	v.T().Logf("Namespace found count: %d", namespaceFoundCount)
	assert.Equal(v.T(), len(rancherPods), namespaceFoundCount,
		fmt.Sprintf("Namespace %s not found in all Rancher pods' databases. Found in %d out of %d pods.",
			namespaceName, namespaceFoundCount, len(rancherPods)))
	v.T().Log("TestCheckNamespaceInAllVAIDatabases test completed")
}

func TestVaiTestSuite(t *testing.T) {
	suite.Run(t, new(VaiTestSuite))
}
