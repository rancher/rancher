/*
The cleanup is designed to get a cluster that was imported to Rancher disconnected
and cleanup some objects

Remove the cattle-system namespace which houses the agent used to talk
to Rancher

Remove Rancher labels, finalizers and annotations from all namespaces

If the cluster was imported to Rancher 2.1 or later cleanup roles, rolebindings,
clusterRoles and clusterRoleBindings
*/

package clean

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/controllers/user/helm"
	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	"github.com/sirupsen/logrus"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	// List of namespace labels that will be removed
	nsLabels = []string{
		nslabels.ProjectIDFieldLabel,
	}

	// List of namespace annotations that will be removed
	nsAnnotations = []string{
		"cattle.io/status",
		"field.cattle.io/creatorId",
		"field.cattle.io/resourceQuotaTemplateId",
		"lifecycle.cattle.io/create.namespace-auth",
		nslabels.ProjectIDFieldLabel,
		helm.AppIDsLabel,
	}

	dryRun bool
)

func Cluster() error {
	if os.Getenv("DRY_RUN") == "true" {
		dryRun = true
	}

	if os.Getenv("SLEEP_FIRST") == "true" {
		// The sleep allows Rancher server time to finish updating ownerReferences
		// and close the connection.
		logrus.Info("Starting sleep for 1 min to allow server time to disconnect....")
		time.Sleep(time.Duration(1) * time.Minute)
	}

	logrus.Info("Starting cluster cleanup")
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	var errors []error

	err = removeCattleNamespace(client)
	if err != nil {
		errors = append(errors, err)
	}

	nsErr := cleanupNamespaces(client)
	if len(nsErr) > 0 {
		errors = append(errors, nsErr...)
	}

	crbErr := cleanupClusterRoleBindings(client)
	if len(crbErr) > 0 {
		errors = append(errors, crbErr...)
	}

	cbErr := cleanupRoleBindings(client)
	if len(cbErr) > 0 {
		errors = append(errors, cbErr...)
	}

	crErr := cleanupClusterRoles(client)
	if len(crErr) > 0 {
		errors = append(errors, crErr...)
	}

	rErr := cleanupRoles(client)
	if len(rErr) > 0 {
		errors = append(errors, rErr...)
	}

	if len(errors) > 0 {
		return processErrors(errors)
	}

	return deleteJob(client)
}

func removeCattleNamespace(client *kubernetes.Clientset) error {
	logrus.Info("Attempting to remove cattle-system namespace")
	return tryUpdate(func() error {
		ns, err := client.CoreV1().Namespaces().Get("cattle-system", metav1.GetOptions{})
		if err != nil {
			if apierror.IsNotFound(err) {
				return nil
			}
			return err
		}
		if len(ns.Finalizers) > 0 {
			ns.Finalizers = []string{}
		}

		logrus.Infof("Updating namespace: %v", ns.Name)
		if !dryRun {
			ns, err = client.CoreV1().Namespaces().Update(ns)
			if err != nil {
				return err
			}
		}

		logrus.Infof("Deleting namespace: %v", ns.Name)
		if !dryRun {
			err = client.CoreV1().Namespaces().Delete("cattle-system", &metav1.DeleteOptions{})
			if err != nil {
				if !apierror.IsNotFound(err) {
					return err
				}
			}
		}

		return nil
	})
}

var listOptions metav1.ListOptions = metav1.ListOptions{
	LabelSelector: "cattle.io/creator=norman",
}

func cleanupNamespaces(client *kubernetes.Clientset) []error {
	logrus.Info("Starting cleanup of namespaces")
	namespaces, err := client.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return []error{err}
	}

	var errs []error

	for _, ns := range namespaces.Items {
		err = tryUpdate(func() error {
			nameSpace, err := client.CoreV1().Namespaces().Get(ns.Name, metav1.GetOptions{})
			if err != nil {
				if apierror.IsNotFound(err) {
					return nil
				}
				return err
			}

			var updated bool

			// Cleanup finalizers
			if len(nameSpace.Finalizers) > 0 {
				finalizers := []string{}
				for _, finalizer := range nameSpace.Finalizers {
					if finalizer != "controller.cattle.io/namespace-auth" {
						finalizers = append(finalizers, finalizer)
					}
				}
				if len(nameSpace.Finalizers) != len(finalizers) {
					updated = true
					nameSpace.Finalizers = finalizers
				}
			}

			// Cleanup labels
			for _, label := range nsLabels {
				if _, ok := nameSpace.Labels[label]; ok {
					updated = ok
					delete(nameSpace.Labels, label)
				}
			}

			// Cleanup annotations
			for _, anno := range nsAnnotations {
				if _, ok := nameSpace.Annotations[anno]; ok {
					updated = ok
					delete(nameSpace.Annotations, anno)
				}
			}

			if updated {
				logrus.Infof("Updating namespace: %v", nameSpace.Name)
				if !dryRun {
					_, err = client.CoreV1().Namespaces().Update(nameSpace)
					if err != nil {
						return err
					}
				}
			}

			return nil
		})

		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs

}

func cleanupClusterRoleBindings(client *kubernetes.Clientset) []error {
	logrus.Info("Starting cleanup of clusterRoleBindings")
	crbs, err := client.RbacV1().ClusterRoleBindings().List(listOptions)
	if err != nil {
		return []error{err}
	}

	var errs []error

	for _, crb := range crbs.Items {
		logrus.Infof("Deleting clusterRoleBinding: %v", crb.Name)
		if !dryRun {
			err = client.RbacV1().ClusterRoleBindings().Delete(crb.Name, &metav1.DeleteOptions{})
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

func cleanupRoleBindings(client *kubernetes.Clientset) []error {
	logrus.Info("Starting cleanup of roleBindings")
	rbs, err := client.RbacV1().RoleBindings("").List(listOptions)
	if err != nil {
		return []error{err}
	}

	var errs []error

	for _, rb := range rbs.Items {
		logrus.Infof("Deleting roleBinding: %v", rb.Name)
		if !dryRun {
			err = client.RbacV1().RoleBindings(rb.Namespace).Delete(rb.Name, &metav1.DeleteOptions{})
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

func cleanupClusterRoles(client *kubernetes.Clientset) []error {
	logrus.Info("Starting cleanup of clusterRoles")
	crs, err := client.RbacV1().ClusterRoles().List(listOptions)
	if err != nil {
		return []error{err}
	}

	var errs []error

	for _, cr := range crs.Items {
		logrus.Infof("Deleting clusterRole: %v", cr.Name)
		if !dryRun {
			err = client.RbacV1().ClusterRoles().Delete(cr.Name, &metav1.DeleteOptions{})
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

func cleanupRoles(client *kubernetes.Clientset) []error {
	logrus.Info("Starting cleanup of roles")
	rs, err := client.RbacV1().Roles("").List(listOptions)
	if err != nil {
		return []error{err}
	}

	var errs []error

	for _, r := range rs.Items {
		logrus.Infof("Deleting role: %v", r.Name)
		if !dryRun {
			err = client.RbacV1().Roles(r.Namespace).Delete(r.Name, &metav1.DeleteOptions{})
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

func deleteJob(client *kubernetes.Clientset) error {
	logrus.Info("Starting cleanup of jobs")
	jobs, err := client.BatchV1().Jobs("default").List(listOptions)
	if err != nil {
		return err
	}

	for _, job := range jobs.Items {
		prop := metav1.DeletePropagationBackground
		if strings.HasPrefix(job.Name, "cattle-cleanup") {
			logrus.Infof("Deleting job: %v", job.Name)
			if !dryRun {
				err = client.BatchV1().Jobs("default").Delete(job.Name, &metav1.DeleteOptions{
					PropagationPolicy: &prop,
				})
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// tryUpdate runs the input func and if the error returned is a conflict error
// from k8s it will sleep and attempt to run the func again. This is useful
// when attempting to update an object.
func tryUpdate(f func() error) error {
	sleepTime := 100
	tries := 0
	var err error
	for tries <= 3 {
		err = f()
		if err != nil {
			if apierror.IsConflict(err) {
				time.Sleep(time.Duration(sleepTime) * time.Millisecond)
				sleepTime *= 2
				tries++
				continue
			}
			return err
		}
		break
	}
	return err
}

func processErrors(errs []error) error {
	errorString := "Errors: "
	for _, err := range errs {
		errorString += fmt.Sprintf("%s ", err)
	}
	return errors.New(errorString)
}
