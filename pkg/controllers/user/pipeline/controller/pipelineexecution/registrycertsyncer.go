package pipelineexecution

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/rke/pki/cert"
	v1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// This Syncer is responsible for certificates rotation for internal registries

const (
	checkCertRotateInterval = 12 * time.Hour
	//rotateThreshold set the rotation threshold so it will rotate at 90% of the total lifetime of the certificates.
	rotateThreshold = 0.9
	maxRetry        = 60
)

type RegistryCertSyncer struct {
	clusterName string

	pods                    v1.PodInterface
	podLister               v1.PodLister
	secrets                 v1.SecretInterface
	secretLister            v1.SecretLister
	managementSecretLister  v1.SecretLister
	namespaceLister         v1.NamespaceLister
	pipelineExecutionLister v3.PipelineExecutionLister
	pipelineSettingLister   v3.PipelineSettingLister
}

func (s *RegistryCertSyncer) sync(ctx context.Context, syncInterval time.Duration) {
	for range ticker.Context(ctx, checkCertRotateInterval) {
		s.checkAndRotateCerts(ctx)
	}
}

func (s *RegistryCertSyncer) checkAndRotateCerts(ctx context.Context) {
	labelsSearchSet := labels.Set{utils.PipelineNamespaceLabel: "true"}
	namespaces, err := s.namespaceLister.List("", labels.SelectorFromSet(labelsSearchSet))
	if err != nil {
		logrus.Error(err)
		return
	}
	for _, ns := range namespaces {
		if ns.DeletionTimestamp != nil {
			continue
		}
		secret, err := s.secretLister.Get(ns.Name, utils.RegistryCrtSecretName)
		if apierrors.IsNotFound(err) {
			continue
		} else if err != nil {
			logrus.Error(err)
			return
		}
		if !s.shouldRotate(secret.Data[utils.RegistryCrt]) {
			continue
		}
		projectID := getProjectID(ns)
		if projectID != "" {
			go func() {
				if err := s.rotateCerts(ctx, projectID); err != nil {
					errors.Wrapf(err, "fail to rotate registry certs for %s project", projectID)
				}
			}()
		}
	}

}

func (s *RegistryCertSyncer) shouldRotate(certPEM []byte) bool {
	currentCerts, err := cert.ParseCertsPEM(certPEM)
	if err != nil || len(currentCerts) < 1 {
		return true
	}
	currentCert := currentCerts[0]
	totalDuration := currentCert.NotAfter.Sub(currentCert.NotBefore)
	thresholdTime := currentCert.NotBefore.Add(time.Duration(float64(totalDuration) * rotateThreshold))
	if time.Now().Before(thresholdTime) {
		return false
	}
	return true
}

func (s *RegistryCertSyncer) rotateCerts(ctx context.Context, projectID string) error {
	logrus.Debugf("rotating registry certs for %s project", projectID)

	clusterID, projectID := ref.Parse(projectID)
	//do certificate rotation when no pipeline execution is running
	startTime := time.Now()
	set := labels.Set(map[string]string{utils.PipelineFinishLabel: "false"})
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()
	for range ticker.Context(cancelCtx, time.Minute) {
		runningExecutions, err := s.pipelineExecutionLister.List(projectID, set.AsSelector())
		if err != nil {
			return err
		}
		if len(runningExecutions) == 0 {
			break
		}
		if time.Now().After(startTime.Add(maxRetry * time.Minute)) {
			return errors.New("time out waiting all executions to finish")
		}
	}

	crt, key, err := s.generateCert(clusterID, projectID)
	if err != nil {
		return err
	}
	ns := projectID + utils.PipelineNamespaceSuffix
	crtSecret, err := s.secretLister.Get(ns, utils.RegistryCrtSecretName)
	if err != nil {
		return err
	}
	toUpdate := crtSecret.DeepCopy()
	toUpdate.Data[utils.RegistryCrt] = cert.EncodeCertPEM(crt)
	toUpdate.Data[utils.RegistryKey] = cert.EncodePrivateKeyPEM(key)
	if _, err := s.secrets.Update(toUpdate); err != nil {
		return errors.Wrapf(err, "Error update secret")
	}

	//delete registry pod to use updated certs
	set = labels.Set(map[string]string{utils.LabelKeyApp: utils.RegistryName})
	pods, err := s.podLister.List(ns, set.AsSelector())
	if err != nil {
		return err
	}
	if len(pods) < 1 {
		return nil
	}
	return s.pods.DeleteNamespaced(ns, pods[0].Name, &metav1.DeleteOptions{})
}

func (s *RegistryCertSyncer) generateCert(clusterID, projectID string) (*x509.Certificate, *rsa.PrivateKey, error) {

	//generate domain cert & key if they do not exist
	caSecret, err := s.managementSecretLister.Get(clusterID, utils.RegistryCACrtSecretName)
	if err != nil {
		return nil, nil, err
	}
	ns := projectID + utils.PipelineNamespaceSuffix
	crtRaw := caSecret.Data[utils.RegistryCACrt]
	keyRaw := caSecret.Data[utils.RegistryCAKey]
	caCrt, err := cert.ParseCertsPEM(crtRaw)
	if err != nil || len(caCrt) < 1 {
		return nil, nil, errors.Wrap(err, "invalid pem format")
	}
	caKey, err := cert.ParsePrivateKeyPEM(keyRaw)

	if _, ok := caKey.(*rsa.PrivateKey); !ok || err != nil {
		return nil, nil, errors.Wrap(err, "invalid pem format")
	}
	cfg := cert.Config{
		CommonName:   utils.RegistryName,
		Organization: []string{},
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		AltNames: cert.AltNames{
			DNSNames: []string{
				utils.RegistryName,
				fmt.Sprintf("%s.%s", utils.RegistryName, ns),
				fmt.Sprintf("%s.%s.svc", utils.RegistryName, ns),
			},
		},
	}
	key, err := cert.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	duration := getSigningDuration(s.pipelineSettingLister, projectID)
	crt, err := newSignedCertWithDuration(cfg, duration, key, caCrt[0], caKey.(*rsa.PrivateKey))
	if err != nil {
		return nil, nil, err
	}
	return crt, key, nil
}
