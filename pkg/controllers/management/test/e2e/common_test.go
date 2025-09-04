package e2e

import (
	"context"
	"os"
	"time"

	"github.com/rancher/rancher/pkg/types/config"
	"gopkg.in/check.v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	crdclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
)

func setupNS(name, projectName string, nsClient v1.NamespaceInterface, c *check.C) *corev1.Namespace {
	if _, err := nsClient.Get(context.TODO(), name, metav1.GetOptions{}); err == nil {
		nsList, err := nsClient.List(context.TODO(), metav1.ListOptions{})
		c.Assert(err, check.IsNil)
		nsListMeta, err := meta.ListAccessor(nsList)
		c.Assert(err, check.IsNil)
		nsWatch, err := nsClient.Watch(context.TODO(), metav1.ListOptions{ResourceVersion: nsListMeta.GetResourceVersion()})
		c.Assert(err, check.IsNil)
		defer nsWatch.Stop()

		if err := nsClient.Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
			c.Fatal(err)
		}

	Loop:
		for {
			select {
			case watchEvent := <-nsWatch.ResultChan():
				if watch.Deleted == watchEvent.Type || watch.Modified == watchEvent.Type {
					if ns, ok := watchEvent.Object.(*corev1.Namespace); ok && ns.Name == name {
						for range 10 {
							if ns, err := nsClient.Get(context.TODO(), name, metav1.GetOptions{}); err == nil {
								if ns.Status.Phase == corev1.NamespaceTerminating && len(ns.Spec.Finalizers) == 0 {
									break Loop
								}
							} else {
								break Loop
							}
							time.Sleep(time.Second)
						}
					}
				}
			case <-time.After(15 * time.Second):
				c.Fatalf("Timeout waiting for namesapce to delete")
			}
		}
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"field.cattle.io/projectId": projectName,
			},
		},
	}
	ns, err := nsClient.Create(context.TODO(), ns, metav1.CreateOptions{})
	c.Assert(err, check.IsNil)

	return ns
}

func setupCRD(name, plural, group, kind, version string, scope apiextensionsv1.ResourceScope, crdClient crdclient.CustomResourceDefinitionInterface,
	crdWatch watch.Interface, c *check.C) {
	fullName := plural + "." + group

	if err := crdClient.Delete(context.TODO(), fullName, metav1.DeleteOptions{}); err == nil {
		waitForCRDDeleted(fullName, crdWatch, crdClient, c)
	}

	crd := newCRD(fullName, name, plural, group, kind, version, scope)
	_, err := crdClient.Create(context.TODO(), crd, metav1.CreateOptions{})
	c.Assert(err, check.IsNil)
	waitForCRDEstablished(fullName, crdWatch, crdClient, c)
}

func waitForCRDEstablished(name string, crdWatch watch.Interface, crdClient crdclient.CustomResourceDefinitionInterface, c *check.C) {
	for {
		select {
		case watchEvent := <-crdWatch.ResultChan():
			if watch.Modified == watchEvent.Type || watch.Added == watchEvent.Type {
				if crd, ok := watchEvent.Object.(*apiextensionsv1.CustomResourceDefinition); ok && crd.Name == name {
					got, err := crdClient.Get(context.TODO(), name, metav1.GetOptions{})
					c.Assert(err, check.IsNil)

					for _, c := range got.Status.Conditions {
						if apiextensionsv1.Established == c.Type && apiextensionsv1.ConditionTrue == c.Status {
							return
						}
					}
				}
			}
		case <-time.After(5 * time.Second):
			c.Fatalf("Timeout waiting for CRD %v to be established", name)
		}
	}
}

func waitForCRDDeleted(name string, crdWatch watch.Interface, crdClient crdclient.CustomResourceDefinitionInterface, c *check.C) {
Loop:
	for {
		select {
		case watchEvent := <-crdWatch.ResultChan():
			if watch.Deleted == watchEvent.Type {
				if crd, ok := watchEvent.Object.(*apiextensionsv1.CustomResourceDefinition); ok && crd.Name == name {
					break Loop
				}
			}
		case <-time.After(5 * time.Second):
			c.Fatalf("timeout waiting for CRD %v to be deleted", name)
		}
	}
}

func newCRD(fullName, name, plural, group, kind, version string, scope apiextensionsv1.ResourceScope) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: fullName,
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: group,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    version,
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type:                   "object",
							XPreserveUnknownFields: &[]bool{true}[0],
						},
					},
				},
			},
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   plural,
				Singular: name,
				Kind:     kind,
			},
			Scope: scope,
		},
	}
}

func clientForSetup(c *check.C) (*clientset.Clientset, *extclient.Clientset, *config.UserContext) {
	mgrConfig := os.Getenv("TEST_CLUSTER_MGR_CONFIG")
	clusterKubeConfig, err := clientcmd.BuildConfigFromFlags("", mgrConfig)
	c.Assert(err, check.IsNil)

	extensionClient, err := extclient.NewForConfig(clusterKubeConfig)
	c.Assert(err, check.IsNil)

	conf := os.Getenv("TEST_CLUSTER_CONFIG")
	workloadKubeConfig, err := clientcmd.BuildConfigFromFlags("", conf)
	c.Assert(err, check.IsNil)

	clusterClient, err := clientset.NewForConfig(workloadKubeConfig)
	c.Assert(err, check.IsNil)

	scaledContext, err := config.NewScaledContext(*clusterKubeConfig, nil)
	c.Assert(err, check.IsNil)

	workload, err := config.NewUserContext(scaledContext, *workloadKubeConfig, "")
	c.Assert(err, check.IsNil)

	return clusterClient, extensionClient, workload
}

func watchChecker(watcher watch.Interface, c *check.C, checker func(watchEvent watch.Event) bool) {
	for {
		select {
		case watchEvent := <-watcher.ResultChan():
			if checker(watchEvent) {
				return
			}
		case <-time.After(5 * time.Second):
			c.Fatalf("Timeout waiting watch condition")
		}
	}
}

func deleteNSOnPass(name string, nsClient v1.NamespaceInterface, c *check.C) {
	if !c.Failed() {
		if err := nsClient.Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
			c.Logf("Error deleting ns %v: %v", name, err)
		}
	}
}
