package machinedrain

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubectl/pkg/drain"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type handler struct {
	ctx          context.Context
	machineCache capicontrollers.MachineCache
	secrets      corecontrollers.SecretClient
	secretCache  corecontrollers.SecretCache
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		ctx:          ctx,
		machineCache: clients.CAPI.Machine().Cache(),
		secrets:      clients.Core.Secret(),
		secretCache:  clients.Core.Secret().Cache(),
	}

	clients.Core.Secret().OnChange(ctx, "machine-drain", h.OnChange)
}

func (h *handler) OnChange(_ string, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil || secret.DeletionTimestamp != nil || secret.Labels[capr.MachineNameLabel] == "" || secret.Type != capr.SecretTypeMachinePlan {
		return secret, nil
	}

	machine, err := h.machineCache.Get(secret.Namespace, secret.Labels[capr.MachineNameLabel])
	if err != nil {
		return secret, err
	}

	oldSecret := secret.DeepCopy()
	defer func() {
		if secret == nil || secret.Annotations == nil {
			secret = oldSecret
		}
		drainErrorValue, ok := secret.Annotations[capr.DrainErrorAnnotation]
		if err == nil && !ok || err != nil && drainErrorValue == err.Error() {
			// No need to update the machine if the annotation is already set to the correct value
			return
		}

		secret = secret.DeepCopy()
		if err != nil {
			secret.Annotations[capr.DrainErrorAnnotation] = err.Error()
		} else {
			delete(secret.Annotations, capr.DrainErrorAnnotation)
		}

		var updateErr error
		if secret, updateErr = h.secrets.Update(secret); updateErr != nil && err == nil {
			err = updateErr
		} else if updateErr != nil {
			err = fmt.Errorf("failed to update secret (%v) after drain error: %v", updateErr, err)
		}
	}()

	drain := secret.Annotations[capr.DrainAnnotation]
	if drain != "" && secret.Annotations[capr.DrainDoneAnnotation] != drain {
		secret, err = h.drain(secret, machine, drain)
	} else if secret.Annotations[capr.UnCordonAnnotation] != "" {
		// Only check that it's non-blank.  There is no correlation between the drain and unDrain options, meaning
		// that the option values do not need to match.  For drain we track the status by doing the drain annotation
		// and then adding a drain-done annotation with the same value when it's done.  Uncordon is different in that
		// we want the final state to have no annotations.  So when UnCordonAnnotation is set we run and then when
		// it's done we delete it.  So there is no knowledge of what the value should be except that it's set.
		secret, err = h.unDrain(secret, machine, drain)
	}

	return secret, err
}

func (h *handler) k8sClient(machine *capi.Machine) (kubernetes.Interface, error) {
	secret, err := h.secretCache.Get(machine.Namespace, name.SafeConcatName(machine.Spec.ClusterName, "kubeconfig"))
	if err != nil {
		return nil, err
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(secret.Data["value"])
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(restConfig)
}

func (h *handler) unDrain(secret *corev1.Secret, machine *capi.Machine, drainData string) (*corev1.Secret, error) {
	if machine.Status.NodeRef == nil || machine.Status.NodeRef.Name == "" {
		logrus.Debugf("unable to drain machine %s as there is no noderef", machine.Name)
		return secret, nil
	}

	var drainOpts rkev1.DrainOptions
	if err := json.Unmarshal([]byte(drainData), &drainOpts); err != nil {
		return secret, err
	}

	checkPostDrainHooks := checkHookAnnotations(drainData, drainOpts.PostDrainHooks)
	if len(drainOpts.PostDrainHooks) > 0 {
		postDrainAnnDoesNotHaveValue := secretAnnotationDoesNotHaveValue(capr.PostDrainAnnotation, drainData)
		if postDrainAnnDoesNotHaveValue(secret) {
			return h.updateSecretAnnotationIfCheckTrue(secret, capr.PostDrainAnnotation, drainData, postDrainAnnDoesNotHaveValue)
		} else if !checkPostDrainHooks(secret) {
			return secret, nil
		}
	}

	helper, node, err := h.getHelper(machine, drainOpts)
	if err != nil {
		return nil, err
	}

	if err := drain.RunCordonOrUncordon(helper, node, false); err != nil {
		return nil, err
	}

	// Drain/Undrain operations are done so clear all annotations involved
	return h.cleanSecretAnnotationsIfCheckTrue(secret, drainOpts, checkPostDrainHooks)
}

func (h *handler) drain(secret *corev1.Secret, machine *capi.Machine, drainData string) (*corev1.Secret, error) {
	drainOpts := &rkev1.DrainOptions{}
	if err := json.Unmarshal([]byte(drainData), drainOpts); err != nil {
		return nil, err
	}

	if err := h.cordon(machine, drainOpts); err != nil {
		return secret, err
	}

	checkPreDrainHooks := checkHookAnnotations(drainData, drainOpts.PreDrainHooks)
	if len(drainOpts.PreDrainHooks) > 0 {
		preDrainAnnDoesNotHaveValue := secretAnnotationDoesNotHaveValue(capr.PreDrainAnnotation, drainData)
		if preDrainAnnDoesNotHaveValue(secret) {
			return h.updateSecretAnnotationIfCheckTrue(secret, capr.PreDrainAnnotation, drainData, preDrainAnnDoesNotHaveValue)
		} else if !checkPreDrainHooks(secret) {
			return secret, nil
		}
	}

	if drainOpts.Enabled {
		if err := h.performDrain(machine, drainOpts); err != nil {
			return nil, err
		}
	}

	return h.updateSecretAnnotationIfCheckTrue(secret, capr.DrainDoneAnnotation, drainData, checkPreDrainHooks)
}

func (h *handler) cordon(machine *capi.Machine, drainOpts *rkev1.DrainOptions) error {
	if machine.Status.NodeRef == nil || machine.Status.NodeRef.Name == "" {
		return nil
	}

	helper, node, err := h.getHelper(machine, *drainOpts)
	if err != nil {
		return err
	}

	return drain.RunCordonOrUncordon(helper, node, true)
}

func (h *handler) getHelper(machine *capi.Machine, drainOpts rkev1.DrainOptions) (*drain.Helper, *corev1.Node, error) {
	k8s, err := h.k8sClient(machine)
	if err != nil {
		return nil, nil, err
	}

	timeout := drainOpts.Timeout
	if timeout == 0 {
		timeout = 600
	}

	helper := &drain.Helper{
		Ctx:                             h.ctx,
		Client:                          k8s,
		Force:                           drainOpts.Force,
		GracePeriodSeconds:              drainOpts.GracePeriod,
		IgnoreAllDaemonSets:             drainOpts.IgnoreDaemonSets == nil || *drainOpts.IgnoreDaemonSets,
		Timeout:                         time.Duration(timeout) * time.Second,
		DeleteEmptyDirData:              drainOpts.DeleteEmptyDirData,
		DisableEviction:                 drainOpts.DisableEviction,
		SkipWaitForDeleteTimeoutSeconds: drainOpts.SkipWaitForDeleteTimeoutSeconds,
		Out:                             os.Stdout,
		ErrOut:                          os.Stderr,
	}

	node, err := k8s.CoreV1().Nodes().Get(h.ctx, machine.Status.NodeRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	return helper, node, err
}

func (h *handler) performDrain(machine *capi.Machine, drainOpts *rkev1.DrainOptions) error {
	if machine.Status.NodeRef == nil || machine.Status.NodeRef.Name == "" {
		return nil
	}

	helper, node, err := h.getHelper(machine, *drainOpts)
	if err != nil {
		return err
	}

	return drain.RunNodeDrain(helper, node.Name)
}

func (h *handler) updateSecretAnnotationIfCheckTrue(secret *corev1.Secret, annotation, value string, check func(*corev1.Secret) bool) (*corev1.Secret, error) {
	var err error
	return secret, retry.RetryOnConflict(retry.DefaultRetry, func() error {
		secret, err = h.secrets.Get(secret.Namespace, secret.Name, metav1.GetOptions{})
		// If there is an error, the check function passes, or the annotation is already set, then return.
		if err != nil || !check(secret) || secret.Annotations[annotation] == value {
			return err
		}
		secret = secret.DeepCopy()
		secret.Annotations[annotation] = value
		_, err = h.secrets.Update(secret)
		return err
	})
}

func (h *handler) cleanSecretAnnotationsIfCheckTrue(secret *corev1.Secret, drainOpts rkev1.DrainOptions, check func(*corev1.Secret) bool) (*corev1.Secret, error) {
	var err error
	return secret, retry.RetryOnConflict(retry.DefaultRetry, func() error {
		secret, err = h.secrets.Get(secret.Namespace, secret.Name, metav1.GetOptions{})
		// If there is an error, the check function passes, or the annotation is already set, then return.
		if err != nil || !check(secret) {
			return err
		}
		secret = secret.DeepCopy()
		delete(secret.Annotations, capr.PreDrainAnnotation)
		delete(secret.Annotations, capr.PostDrainAnnotation)
		delete(secret.Annotations, capr.DrainAnnotation)
		delete(secret.Annotations, capr.DrainDoneAnnotation)
		delete(secret.Annotations, capr.UnCordonAnnotation)
		for _, hook := range drainOpts.PreDrainHooks {
			delete(secret.Annotations, hook.Annotation)
		}
		for _, hook := range drainOpts.PostDrainHooks {
			delete(secret.Annotations, hook.Annotation)
		}
		_, err = h.secrets.Update(secret)
		return err
	})
}

func secretAnnotationDoesNotHaveValue(annotation, value string) func(*corev1.Secret) bool {
	return func(secret *corev1.Secret) bool {
		return secret.Annotations[annotation] != value
	}
}

func checkHookAnnotations(drainData string, hooks []rkev1.DrainHook) func(secret *corev1.Secret) bool {
	return func(secret *corev1.Secret) bool {
		for _, hook := range hooks {
			if hook.Annotation != "" && secret.Annotations[hook.Annotation] != drainData {
				return false
			}
		}
		return true
	}
}
