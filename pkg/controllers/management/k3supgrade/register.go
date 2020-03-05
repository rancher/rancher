package k3supgrade

import (
	"context"
	"reflect"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerv3 "github.com/rancher/rancher/pkg/wrangler/generated/controllers/management.cattle.io/v3"
	planv1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	planClientset "github.com/rancher/system-upgrade-controller/pkg/generated/clientset/versioned/typed/upgrade.cattle.io/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type handler struct {
	systemUpgradeNamespace string
	clusterCache           wranglerv3.ClusterCache
	clusterClient          wranglerv3.ClusterClient
	apps                   projectv3.AppInterface
	appLister              projectv3.AppLister
	templateLister         v3.CatalogTemplateLister
	systemAccountManager   *systemaccount.Manager
	manager                *clustermanager.Manager
}

const (
	systemUpgradeNS        = "cattle-system"
	rancherManagedPlan     = "rancher-managed"
	upgradeDisableLabelKey = "plan.upgrade.cattle.io/disable"
	k3sUpgraderCatalogName = "system-library-rancher-k3s-upgrader"
)

func Register(ctx context.Context, wContext *wrangler.Context, mgmtCtx *config.ManagementContext, manager *clustermanager.Manager) {
	h := &handler{
		systemUpgradeNamespace: systemUpgradeNS,
		clusterCache:           wContext.Mgmt.Cluster().Cache(),
		clusterClient:          wContext.Mgmt.Cluster(),
		apps:                   mgmtCtx.Project.Apps(metav1.NamespaceAll),
		appLister:              mgmtCtx.Project.Apps("").Controller().Lister(),
		templateLister:         mgmtCtx.Management.CatalogTemplates("").Controller().Lister(),
		systemAccountManager:   systemaccount.NewManager(mgmtCtx),
		manager:                manager,
	}

	wContext.Mgmt.Cluster().OnChange(ctx, "k3s-upgrade-controller", h.onClusterChange)
}

// deployPlans creates a master and worker plan in the downstream cluster to instrument
// the system-upgrade-controller in the downstream cluster
func (h *handler) deployPlans(cluster *v3.Cluster) error {

	// access downstream cluster
	clusterCtx, err := h.manager.UserContext(cluster.Name)
	if err != nil {
		return err

	}

	// create a client for GETing Plans in the downstream cluster
	planConfig, err := planClientset.NewForConfig(&clusterCtx.RESTConfig)
	if err != nil {
		return err
	}
	planClient := planConfig.Plans(metav1.NamespaceAll)

	planList, err := planClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	masterPlan := planv1.Plan{}
	workerPlan := planv1.Plan{}
	// deactivate all existing plans that are not managed by Rancher
	for _, plan := range planList.Items {
		if _, ok := plan.Labels[rancherManagedPlan]; !ok {
			// inverse selection is used here, we select a non-existent label
			plan.Spec.NodeSelector = &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      upgradeDisableLabelKey,
					Operator: metav1.LabelSelectorOpExists,
				}}}

			_, err = planClient.Update(&plan)
			if err != nil {
				return err
			}
		} else {
			// if any of the rancher plans are currently applying, set updating status on cluster
			if len(plan.Status.Applying) > 0 {
				v3.ClusterConditionUpdated.Unknown(cluster)
				cluster, err = h.clusterClient.Update(cluster)
				if err != nil {
					return err
				}
			} else {
				//set it back if not
				if v3.ClusterConditionUpdated.IsUnknown(cluster) {
					v3.ClusterConditionUpdated.True(cluster)
					cluster, err = h.clusterClient.Update(cluster)
					if err != nil {
						return err
					}
				}
			}

			switch name := plan.Name; name {
			case k3sMasterPlanName:
				masterPlan = plan
			case k3sWorkerPlanName:
				workerPlan = plan
			}
		}
	}

	// if rancher plans exist, do we need to update?
	if masterPlan.Name != "" || workerPlan.Name != "" {
		if masterPlan.Name != "" {
			newMaster := configureMasterPlan(masterPlan, cluster.Spec.K3sConfig.Version, cluster.Spec.K3sConfig.ServerConcurrency)

			if !cmp(masterPlan, newMaster) {
				planClient = planConfig.Plans(systemUpgradeNS)
				_, err = planClient.Update(&newMaster)
				if err != nil {
					return err
				}
			}
		}

		if workerPlan.Name != "" {
			newWorker := configureWorkerPlan(workerPlan, cluster.Spec.K3sConfig.Version, cluster.Spec.K3sConfig.WorkerConcurrency)

			if !cmp(workerPlan, newWorker) {
				planClient = planConfig.Plans(systemUpgradeNS)
				_, err = planClient.Update(&newWorker)
				if err != nil {
					return err
				}
			}
		}

	} else { // create the plans
		planClient = planConfig.Plans(systemUpgradeNS)
		masterPlan = generateMasterPlan(cluster.Spec.K3sConfig.Version,
			cluster.Spec.K3sConfig.ServerConcurrency)
		_, err = planClient.Create(&masterPlan)
		if err != nil {
			return err
		}
		workerPlan = generateWorkerPlan(cluster.Spec.K3sConfig.Version,
			cluster.Spec.K3sConfig.WorkerConcurrency)
		_, err = planClient.Create(&workerPlan)
		if err != nil {
			return err
		}
		logrus.Infof("Plans successfully deployed into cluster %s", cluster.Name)
	}

	return nil
}

//cmp compares two plans but does not compare their Status, returns true if they are the same
func cmp(a, b planv1.Plan) bool {
	if a.Namespace != b.Namespace {
		return false
	}

	if a.Spec.Version != b.Spec.Version {
		return false
	}

	if a.Spec.Concurrency != b.Spec.Concurrency {
		return false
	}

	//TODO Refactor to not use reflection
	if !reflect.DeepEqual(a.Spec, b.Spec) {
		return false
	}
	if !reflect.DeepEqual(a.ObjectMeta, b.ObjectMeta) {
		return false
	}
	if !reflect.DeepEqual(a.TypeMeta, b.TypeMeta) {
		return false
	}
	return true
}
