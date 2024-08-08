//go:build !windows
// +build !windows

// Clean duplicated PRTBs found in a management cluster. This will collect all
// PRTBs and check for duplicates. If they are found delete all but 1.

package clean

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	rbacv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac"
	"github.com/rancher/wrangler/v3/pkg/ratelimit"
	"github.com/rancher/wrangler/v3/pkg/start"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	dupePRTBsOperation = "clean-dupe-prtbs"
)

type dupePRTBsCleanup struct {
	prtbs v3.ProjectRoleTemplateBindingClient
}

type projectRoleTemplateBindingDuplicate struct {
	PRTBs []prtbLight
}

type prtbLight struct {
	Namespace         string
	Name              string
	CreationTimestamp time.Time
}

func DuplicatePRTBs(clientConfig *restclient.Config) error {
	logrus.Infof("[%v] starting prtb cleanup", dupePRTBsOperation)
	if os.Getenv("DRY_RUN") == "true" {
		logrus.Infof("[%v] DRY_RUN is true, no objects will be deleted/modified", dupePRTBsOperation)
		dryRun = true
	}

	var config *restclient.Config
	var err error
	if clientConfig != nil {
		config = clientConfig
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		if err != nil {
			logrus.Errorf("[%v] error in building the cluster config %v", dupeBindingsOperation, err)
			return err
		}
	}
	// No one wants to be slow
	config.RateLimiter = ratelimit.None

	rancherManagement, err := management.NewFactoryFromConfig(config)
	if err != nil {
		return err
	}

	k8srbac, err := rbac.NewFactoryFromConfig(config)
	if err != nil {
		return err
	}

	starters := []start.Starter{rancherManagement, k8srbac}

	ctx := context.Background()
	if err := start.All(ctx, 5, starters...); err != nil {
		return err
	}

	pc := dupePRTBsCleanup{
		prtbs: rancherManagement.Management().V3().ProjectRoleTemplateBinding(),
	}

	return pc.clean()
}

func (pc *dupePRTBsCleanup) clean() error {
	prtbs, err := pc.prtbs.List("", metav1.ListOptions{})
	if err != nil {
		return err
	}

	dupsMap := analyzeProjectRoleTemplateBindings(prtbs.Items)
	dupsMap = findAndOrderDuplicates(dupsMap)

	logrus.Infof("[%v] delete duplicates", dupePRTBsOperation)

	for _, prtbSet := range dupsMap {
		for i, dup := range prtbSet.PRTBs {
			// skip first to keep the oldest
			if i == 0 {
				continue
			}

			// delete everything else
			logrus.Infof("[%v] deleting %s.%s", dupePRTBsOperation, dup.Namespace, dup.Name)
			pc.prtbs.Delete(dup.Namespace, dup.Name, &metav1.DeleteOptions{})
		}
	}

	return nil
}

func analyzeProjectRoleTemplateBindings(prtbs []rbacv3.ProjectRoleTemplateBinding) map[string]projectRoleTemplateBindingDuplicate {
	logrus.Infof("[%v] analyzing project role template bindings", dupePRTBsOperation)

	hashes := make(map[string]projectRoleTemplateBindingDuplicate)

	for _, prtb := range prtbs {
		hash := getPRTBHash(prtb)

		entry, found := hashes[hash]
		// initialize entry on first access
		if !found {
			entry.PRTBs = []prtbLight{}
		}

		entry.PRTBs = append(entry.PRTBs, prtbLight{
			Namespace:         prtb.ObjectMeta.Namespace,
			Name:              prtb.ObjectMeta.Name,
			CreationTimestamp: prtb.ObjectMeta.CreationTimestamp.Time,
		})

		hashes[hash] = entry
	}

	return hashes
}

func findAndOrderDuplicates(all map[string]projectRoleTemplateBindingDuplicate) map[string]projectRoleTemplateBindingDuplicate {
	logrus.Infof("[%v] find duplicates and order PRTBs", dupePRTBsOperation)

	for k, prtb := range all {
		if len(prtb.PRTBs) == 1 {
			delete(all, k)
			continue
		}

		slices.SortFunc(prtb.PRTBs, func(a, b prtbLight) int {
			return int(a.CreationTimestamp.Unix() - b.CreationTimestamp.Unix())
		})
	}

	return all
}

func getPRTBHash(prtb rbacv3.ProjectRoleTemplateBinding) string {
	return fmt.Sprintf(
		"%s,%s,%s,%s",
		prtb.ObjectMeta.Namespace,
		prtb.ProjectName,
		prtb.RoleTemplateName,
		prtb.UserPrincipalName,
	)
}
