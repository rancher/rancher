package management

import (
	"context"
	"reflect"
	"sort"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/util/retry"
)

const (
	bootstrappedRole       = "authz.management.cattle.io/bootstrapped-role"
	bootstrapAdminConfig   = "admincreated"
	cattleNamespace        = "cattle-system"
	defaultAdminLabelKey   = "authz.management.cattle.io/bootstrapping"
	defaultAdminLabelValue = "admin-user"
)

var (
	defaultAdminLabel = map[string]string{defaultAdminLabelKey: defaultAdminLabelValue}
	adminCreateLock   sync.Mutex
)

func addRoles(wrangler *wrangler.Context, management *config.ManagementContext) (string, error) {
	rb := newRoleBuilder()

	rb.addRole("Create Clusters", "clusters-create").
		addRule().apiGroups("management.cattle.io").resources("clusters").verbs("create").
		addRule().apiGroups("provisioning.cattle.io").resources("clusters").verbs("create").
		addRule().apiGroups("management.cattle.io").resources("templates", "templateversions").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("nodedrivers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("kontainerdrivers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("podsecuritypolicytemplates").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("podsecurityadmissionconfigurationtemplates").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("nodetemplates").verbs("*").
		addRule().apiGroups("").resources("secrets").verbs("create").
		addRule().apiGroups("management.cattle.io").resources("cisconfigs").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("cisbenchmarkversions").verbs("get", "list", "watch").
		addRule().apiGroups("rke-machine-config.cattle.io").resources("*").verbs("create").
		addRule().apiGroups("catalog.cattle.io").resources("clusterrepos").verbs("get", "list", "watch").
		addRule().apiGroups("rke.cattle.io").resources("etcdsnapshots").verbs("get", "list", "watch")

	rb.addRole("Manage Node Drivers", "nodedrivers-manage").
		addRule().apiGroups("management.cattle.io").resources("nodedrivers").verbs("*")
	rb.addRole("Manage Cluster Drivers", "kontainerdrivers-manage").
		addRule().apiGroups("management.cattle.io").resources("kontainerdrivers").verbs("*")
	rb.addRole("Manage Catalogs", "catalogs-manage").
		addRule().apiGroups("management.cattle.io").resources("catalogs", "templates", "templateversions").verbs("*")
	rb.addRole("Use Catalog Templates", "catalogs-use").
		addRule().apiGroups("management.cattle.io").resources("templates", "templateversions").verbs("get", "list", "watch")
	rb.addRole("Manage Users", "users-manage").
		addRule().apiGroups("management.cattle.io").resources("users", "globalrolebindings").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("globalroles").verbs("get", "list", "watch")
	rb.addRole("Manage Roles", "roles-manage").
		addRule().apiGroups("management.cattle.io").resources("roletemplates").verbs("delete", "deletecollection", "get", "list", "patch", "create", "update", "watch")
	rb.addRole("Manage Authentication", "authn-manage").
		addRule().apiGroups("management.cattle.io").resources("authconfigs").verbs("get", "list", "watch", "update")
	rb.addRole("Manage Settings", "settings-manage").
		addRule().apiGroups("management.cattle.io").resources("settings").verbs("*")
	rb.addRole("Manage Features", "features-manage").
		addRule().apiGroups("management.cattle.io").resources("features").verbs("get", "list", "watch", "update")
	rb.addRole("Manage PodSecurityPolicy Templates", "podsecuritypolicytemplates-manage").
		addRule().apiGroups("management.cattle.io").resources("podsecuritypolicytemplates").verbs("*")
	rb.addRole("Create RKE Templates", "clustertemplates-create").
		addRule().apiGroups("management.cattle.io").resources("clustertemplates").verbs("create")
	rb.addRole("Create RKE Template Revisions", "clustertemplaterevisions-create").
		addRule().apiGroups("management.cattle.io").resources("clustertemplaterevisions").verbs("create")
	rb.addRole("View Rancher Metrics", "view-rancher-metrics").
		addRule().apiGroups("management.cattle.io").resources("ranchermetrics").verbs("get")

	rb.addRole("Admin", "admin").
		addRule().apiGroups("*").resources("*").verbs("*").
		addRule().apiGroups().nonResourceURLs("*").verbs("*")

	// restricted-admin will get cluster admin access to all downstream clusters but limited access to the local cluster
	restrictedAdminRole := addUserRules(rb.addRole("Restricted Admin", "restricted-admin"))
	restrictedAdminRole.
		addRule().apiGroups("catalog.cattle.io").resources("clusterrepos").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("clustertemplates").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("clustertemplaterevisions").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("globalrolebindings").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("globalroles").verbs("delete", "deletecollection", "get", "list", "patch", "create", "update", "watch").
		addRule().apiGroups("management.cattle.io").resources("users", "userattribute", "groups", "groupmembers").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("podsecuritypolicytemplates").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("podsecurityadmissionconfigurationtemplates").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("fleetworkspaces").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("authconfigs").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("nodedrivers").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("kontainerdrivers").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("roletemplates").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("catalogs", "templates", "templateversions").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("features").verbs("update", "patch", "security-enable").resourceNames("external-rules")

	// restricted-admin can edit settings if rancher is bootstrapped with restricted-admin role
	if settings.RestrictedDefaultAdmin.Get() == "true" {
		restrictedAdminRole.
			addRule().apiGroups("management.cattle.io").resources("settings").verbs("*")
	}

	userRole := addUserRules(rb.addRole("User", "user"))
	userRole.
		addRule().apiGroups("catalog.cattle.io").resources("clusterrepos").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("podsecuritypolicytemplates").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("podsecurityadmissionconfigurationtemplates").verbs("get", "list", "watch")

	rb.addRole("User Base", "user-base").
		addRule().apiGroups("management.cattle.io").resources("preferences").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("settings").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("features").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("sourcecodecredentials").verbs("*").
		addRule().apiGroups("project.cattle.io").resources("sourcecoderepositories").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("rancherusernotifications").verbs("get", "list", "watch")

	// TODO user should be dynamically authorized to only see herself
	// TODO enable when groups are "in". they need to be self-service

	if err := rb.reconcileGlobalRoles(wrangler.Mgmt.GlobalRole()); err != nil {
		return "", errors.Wrap(err, "problem reconciling global roles")
	}

	// RoleTemplates to be used inside of clusters
	rb = newRoleBuilder()

	// K8s default roles
	rb.addRoleTemplate("Kubernetes cluster-admin", "cluster-admin", "cluster", true, true, true)
	rb.addRoleTemplate("Kubernetes admin", "admin", "project", true, true, false)
	rb.addRoleTemplate("Kubernetes edit", "edit", "project", true, true, false)
	rb.addRoleTemplate("Kubernetes view", "view", "project", true, true, false)

	// Cluster roles
	rb.addRoleTemplate("Cluster Owner", "cluster-owner", "cluster", false, false, true).
		addRule().apiGroups("*").resources("*").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("clusters").verbs("own").
		addRule().apiGroups("provisioning.cattle.io").resources("clusters").verbs("*").
		addRule().apiGroups("rke.cattle.io").resources("etcdsnapshots").verbs("get", "list", "watch").
		addRule().apiGroups("cluster.x-k8s.io").resources("machines").verbs("*").
		addRule().apiGroups("rke-machine-config.cattle.io").resources("*").verbs("*").
		addRule().apiGroups("rke-machine.cattle.io").resources("*").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("projects").verbs("updatepsa").
		addRule().apiGroups().nonResourceURLs("*").verbs("*")

	rb.addRoleTemplate("Cluster Member", "cluster-member", "cluster", false, false, false).
		addRule().apiGroups("ui.cattle.io").resources("navlinks").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusterroletemplatebindings").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projects").verbs("create").
		addRule().apiGroups("management.cattle.io").resources("nodes", "nodepools").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("nodes").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("persistentvolumes").verbs("get", "list", "watch").
		addRule().apiGroups("storage.k8s.io").resources("storageclasses").verbs("get", "list", "watch").
		addRule().apiGroups("apiregistration.k8s.io").resources("apiservices").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusterloggings").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusteralertrules").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusteralertgroups").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("notifiers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clustercatalogs").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clustermonitorgraphs").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("catalogtemplates").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("catalogtemplateversions").verbs("get", "list", "watch").
		addRule().apiGroups("catalog.cattle.io").resources("clusterrepos").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusters").resourceNames("local").verbs("get").
		addRule().apiGroups("provisioning.cattle.io").resources("clusters").verbs("get", "watch").
		addRule().apiGroups("cluster.x-k8s.io").resources("machines").verbs("get", "watch").
		addRule().apiGroups("cluster.x-k8s.io").resources("machinedeployments").verbs("get", "watch").
		addRule().apiGroups("rke-machine-config.cattle.io").resources("*").verbs("get", "watch").
		addRule().apiGroups("rke-machine.cattle.io").resources("*").verbs("get", "watch").
		addRule().apiGroups("metrics.k8s.io").resources("nodemetrics", "nodes").verbs("get", "list", "watch")

	rb.addRoleTemplate("Create Projects", "projects-create", "cluster", false, false, false).
		addRule().apiGroups("management.cattle.io").resources("projects").verbs("create")

	rb.addRoleTemplate("View All Projects", "projects-view", "cluster", false, false, false).
		addRule().apiGroups("management.cattle.io").resources("projects").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("apps").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("apprevisions").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("namespaces").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("persistentvolumes").verbs("get", "list", "watch").
		addRule().apiGroups("storage.k8s.io").resources("storageclasses").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("persistentvolumeclaims").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch").
		setRoleTemplateNames("view")

	rb.addRoleTemplate("Manage Nodes", "nodes-manage", "cluster", false, false, false).
		addRule().apiGroups("management.cattle.io").resources("nodes", "nodepools").verbs("*").
		addRule().apiGroups("").resources("nodes").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("clustermonitorgraphs").verbs("get", "list", "watch").
		addRule().apiGroups("cluster.x-k8s.io").resources("machines").verbs("*").
		addRule().apiGroups("cluster.x-k8s.io").resources("machinedeployments").verbs("*").
		addRule().apiGroups("rke-machine-config.cattle.io").resources("*").verbs("*").
		addRule().apiGroups("rke-machine.cattle.io").resources("*").verbs("*")

	rb.addRoleTemplate("View Nodes", "nodes-view", "cluster", false, false, false).
		addRule().apiGroups("management.cattle.io").resources("nodes", "nodepools").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("nodes").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clustermonitorgraphs").verbs("get", "list", "watch").
		addRule().apiGroups("cluster.x-k8s.io").resources("machines").verbs("get", "watch").
		addRule().apiGroups("cluster.x-k8s.io").resources("machinedeployments").verbs("get", "watch").
		addRule().apiGroups("rke-machine-config.cattle.io").resources("*").verbs("get", "watch").
		addRule().apiGroups("rke-machine.cattle.io").resources("*").verbs("get", "watch")

	rb.addRoleTemplate("Manage Storage", "storage-manage", "cluster", false, false, false).
		addRule().apiGroups("").resources("persistentvolumes").verbs("*").
		addRule().apiGroups("storage.k8s.io").resources("storageclasses").verbs("*").
		addRule().apiGroups("").resources("persistentvolumeclaims").verbs("*")

	rb.addRoleTemplate("Manage Cluster Members", "clusterroletemplatebindings-manage", "cluster", false, false, false).
		addRule().apiGroups("management.cattle.io").resources("clusterroletemplatebindings").verbs("*")

	rb.addRoleTemplate("View Cluster Members", "clusterroletemplatebindings-view", "cluster", false, false, false).
		addRule().apiGroups("management.cattle.io").resources("clusterroletemplatebindings").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Cluster Catalogs", "clustercatalogs-manage", "cluster", false, false, true).
		addRule().apiGroups("management.cattle.io").resources("clustercatalogs").verbs("*").
		addRule().apiGroups("catalog.cattle.io").resources("clusterrepos").verbs("*")

	rb.addRoleTemplate("View Cluster Catalogs", "clustercatalogs-view", "cluster", false, false, false).
		addRule().apiGroups("management.cattle.io").resources("clustercatalogs").verbs("get", "list", "watch").
		addRule().apiGroups("catalog.cattle.io").resources("clusterrepos").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Cluster Backups", "backups-manage", "cluster", false, false, false).
		addRule().apiGroups("management.cattle.io").resources("etcdbackups").verbs("*")

	rb.addRoleTemplate("Manage Navlinks", "navlinks-manage", "cluster", false, false, false).
		addRule().apiGroups("ui.cattle.io").resources("navlinks").verbs("*")

	// Project roles
	rb.addRoleTemplate("Project Owner", "project-owner", "project", false, false, false).
		addRule().apiGroups("ui.cattle.io").resources("navlinks").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("nodes").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("*").
		addRule().apiGroups("project.cattle.io").resources("apps").verbs("*").
		addRule().apiGroups("project.cattle.io").resources("apprevisions").verbs("*").
		addRule().apiGroups("project.cattle.io").resources("sourcecodeproviderconfigs").verbs("*").
		addRule().apiGroups("").resources("namespaces").verbs("create").
		addRule().apiGroups("").resources("persistentvolumes").verbs("get", "list", "watch").
		addRule().apiGroups("storage.k8s.io").resources("storageclasses").verbs("get", "list", "watch").
		addRule().apiGroups("apiregistration.k8s.io").resources("apiservices").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("persistentvolumeclaims").verbs("*").
		addRule().apiGroups("metrics.k8s.io").resources("pods").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("notifiers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectalertrules").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("projectalertgroups").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("projectloggings").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("clustercatalogs").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectcatalogs").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("projectmonitorgraphs").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("catalogtemplates").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("catalogtemplateversions").verbs("*").
		addRule().apiGroups("monitoring.cattle.io").resources("prometheus").verbs("view").
		addRule().apiGroups("monitoring.coreos.com").resources("prometheuses", "prometheusrules", "servicemonitors").verbs("*").
		addRule().apiGroups("networking.istio.io").resources("destinationrules", "envoyfilters", "gateways", "serviceentries", "sidecars", "virtualservices").verbs("*").
		addRule().apiGroups("config.istio.io").resources("apikeys", "authorizations", "checknothings", "circonuses", "deniers", "fluentds", "handlers", "kubernetesenvs", "kuberneteses", "listcheckers", "listentries", "logentries", "memquotas", "metrics", "opas", "prometheuses", "quotas", "quotaspecbindings", "quotaspecs", "rbacs", "reportnothings", "rules", "solarwindses", "stackdrivers", "statsds", "stdios").verbs("*").
		addRule().apiGroups("authentication.istio.io").resources("policies").verbs("*").
		addRule().apiGroups("rbac.istio.io").resources("rbacconfigs", "serviceroles", "servicerolebindings").verbs("*").
		addRule().apiGroups("security.istio.io").resources("authorizationpolicies").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("projects").verbs("own").
		addRule().apiGroups("catalog.cattle.io").resources("clusterrepos").verbs("get", "list", "watch").
		addRule().apiGroups("catalog.cattle.io").resources("operations").verbs("get", "list", "watch").
		addRule().apiGroups("catalog.cattle.io").resources("releases").verbs("get", "list", "watch").
		addRule().apiGroups("catalog.cattle.io").resources("apps").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusters").verbs("get").resourceNames("local").
		setRoleTemplateNames("admin")

	rb.addRoleTemplate("Project Member", "project-member", "project", false, false, false).
		addRule().apiGroups("ui.cattle.io").resources("navlinks").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("apps").verbs("*").
		addRule().apiGroups("project.cattle.io").resources("apprevisions").verbs("*").
		addRule().apiGroups("").resources("namespaces").verbs("create").
		addRule().apiGroups("").resources("persistentvolumes").verbs("get", "list", "watch").
		addRule().apiGroups("storage.k8s.io").resources("storageclasses").verbs("get", "list", "watch").
		addRule().apiGroups("apiregistration.k8s.io").resources("apiservices").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("persistentvolumeclaims").verbs("*").
		addRule().apiGroups("metrics.k8s.io").resources("pods").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("notifiers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectalertrules").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("projectalertgroups").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("projectloggings").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clustercatalogs").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectcatalogs").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectmonitorgraphs").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("catalogtemplates").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("catalogtemplateversions").verbs("get", "list", "watch").
		addRule().apiGroups("monitoring.cattle.io").resources("prometheus").verbs("view").
		addRule().apiGroups("monitoring.coreos.com").resources("prometheuses", "prometheusrules", "servicemonitors").verbs("*").
		addRule().apiGroups("networking.istio.io").resources("destinationrules", "envoyfilters", "gateways", "serviceentries", "sidecars", "virtualservices").verbs("*").
		addRule().apiGroups("config.istio.io").resources("apikeys", "authorizations", "checknothings", "circonuses", "deniers", "fluentds", "handlers", "kubernetesenvs", "kuberneteses", "listcheckers", "listentries", "logentries", "memquotas", "metrics", "opas", "prometheuses", "quotas", "quotaspecbindings", "quotaspecs", "rbacs", "reportnothings", "rules", "solarwindses", "stackdrivers", "statsds", "stdios").verbs("*").
		addRule().apiGroups("authentication.istio.io").resources("policies").verbs("*").
		addRule().apiGroups("rbac.istio.io").resources("rbacconfigs", "serviceroles", "servicerolebindings").verbs("*").
		addRule().apiGroups("security.istio.io").resources("authorizationpolicies").verbs("*").
		addRule().apiGroups("catalog.cattle.io").resources("clusterrepos").verbs("get", "list", "watch").
		addRule().apiGroups("catalog.cattle.io").resources("operations").verbs("get", "list", "watch").
		addRule().apiGroups("catalog.cattle.io").resources("releases").verbs("get", "list", "watch").
		addRule().apiGroups("catalog.cattle.io").resources("apps").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusters").verbs("get").resourceNames("local").
		setRoleTemplateNames("edit")

	rb.addRoleTemplate("Read-only", "read-only", "project", false, false, false).
		addRule().apiGroups("ui.cattle.io").resources("navlinks").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("apps").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("apprevisions").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("persistentvolumes").verbs("get", "list", "watch").
		addRule().apiGroups("storage.k8s.io").resources("storageclasses").verbs("get", "list", "watch").
		addRule().apiGroups("apiregistration.k8s.io").resources("apiservices").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("persistentvolumeclaims").verbs("get", "list", "watch").
		addRule().apiGroups("metrics.k8s.io").resources("pods").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("notifiers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectalertrules").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectalertgroups").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectloggings").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clustercatalogs").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectcatalogs").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectmonitorgraphs").verbs("get", "list", "watch").
		addRule().apiGroups("monitoring.coreos.com").resources("prometheuses", "prometheusrules", "servicemonitors").verbs("get", "list", "watch").
		addRule().apiGroups("networking.istio.io").resources("destinationrules", "envoyfilters", "gateways", "serviceentries", "sidecars", "virtualservices").verbs("get", "list", "watch").
		addRule().apiGroups("config.istio.io").resources("apikeys", "authorizations", "checknothings", "circonuses", "deniers", "fluentds", "handlers", "kubernetesenvs", "kuberneteses", "listcheckers", "listentries", "logentries", "memquotas", "metrics", "opas", "prometheuses", "quotas", "quotaspecbindings", "quotaspecs", "rbacs", "reportnothings", "rules", "solarwindses", "stackdrivers", "statsds", "stdios").verbs("get", "list", "watch").
		addRule().apiGroups("authentication.istio.io").resources("policies").verbs("get", "list", "watch").
		addRule().apiGroups("rbac.istio.io").resources("rbacconfigs", "serviceroles", "servicerolebindings").verbs("get", "list", "watch").
		addRule().apiGroups("security.istio.io").resources("authorizationpolicies").verbs("get", "list", "watch").
		addRule().apiGroups("catalog.cattle.io").resources("clusterrepos").verbs("get", "list", "watch").
		addRule().apiGroups("catalog.cattle.io").resources("operations").verbs("get", "list", "watch").
		addRule().apiGroups("catalog.cattle.io").resources("releases").verbs("get", "list", "watch").
		addRule().apiGroups("catalog.cattle.io").resources("apps").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusters").verbs("get").resourceNames("local").
		setRoleTemplateNames("view")

	rb.addRoleTemplate("Create Namespaces", "create-ns", "project", false, false, false).
		addRule().apiGroups("").resources("namespaces").verbs("create")

	rb.addRoleTemplate("Manage Workloads", "workloads-manage", "project", false, false, false).
		addRule().apiGroups("").resources("pods", "pods/attach", "pods/exec", "pods/portforward", "pods/proxy", "replicationcontrollers",
		"replicationcontrollers/scale").verbs("*").
		addRule().apiGroups("apps").resources("daemonsets", "deployments", "deployments/rollback", "deployments/scale", "replicasets",
		"replicasets/scale", "statefulsets", "statefulsets/scale").verbs("*").
		addRule().apiGroups("autoscaling").resources("horizontalpodautoscalers").verbs("*").
		addRule().apiGroups("batch").resources("cronjobs", "jobs").verbs("*").
		addRule().apiGroups("").resources("limitranges", "pods/log", "pods/status", "replicationcontrollers/status", "resourcequotas", "resourcequotas/status", "bindings").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("apps").verbs("*").
		addRule().apiGroups("project.cattle.io").resources("apprevisions").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("projectmonitorgraphs").verbs("get", "list", "watch")

	rb.addRoleTemplate("View Workloads", "workloads-view", "project", false, false, false).
		addRule().apiGroups("").resources("pods", "replicationcontrollers", "replicationcontrollers/scale").verbs("get", "list", "watch").
		addRule().apiGroups("apps").resources("daemonsets", "deployments", "deployments/rollback", "deployments/scale", "replicasets",
		"replicasets/scale", "statefulsets").verbs("get", "list", "watch").
		addRule().apiGroups("autoscaling").resources("horizontalpodautoscalers").verbs("get", "list", "watch").
		addRule().apiGroups("batch").resources("cronjobs", "jobs").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("limitranges", "pods/log", "pods/status", "replicationcontrollers/status", "resourcequotas", "resourcequotas/status", "bindings").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("apps").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("apprevisions").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectmonitorgraphs").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Ingress", "ingress-manage", "project", false, false, false).
		addRule().apiGroups("extensions").resources("ingresses").verbs("*").
		addRule().apiGroups("networking.k8s.io").resources("ingresses").verbs("*")

	rb.addRoleTemplate("View Ingress", "ingress-view", "project", false, false, false).
		addRule().apiGroups("extensions").resources("ingresses").verbs("get", "list", "watch").
		addRule().apiGroups("networking.k8s.io").resources("ingresses").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Services", "services-manage", "project", false, false, false).
		addRule().apiGroups("").resources("services", "services/proxy", "endpoints").verbs("*")

	rb.addRoleTemplate("View Services", "services-view", "project", false, false, false).
		addRule().apiGroups("").resources("services", "endpoints").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Secrets", "secrets-manage", "project", false, false, false).
		addRule().apiGroups("").resources("secrets").verbs("*")

	rb.addRoleTemplate("View Secrets", "secrets-view", "project", false, false, false).
		addRule().apiGroups("").resources("secrets").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Config Maps", "configmaps-manage", "project", false, false, false).
		addRule().apiGroups("").resources("configmaps").verbs("*")

	rb.addRoleTemplate("View Config Maps", "configmaps-view", "project", false, false, false).
		addRule().apiGroups("").resources("configmaps").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Volumes", "persistentvolumeclaims-manage", "project", false, false, false).
		addRule().apiGroups("").resources("persistentvolumes").verbs("get", "list", "watch").
		addRule().apiGroups("storage.k8s.io").resources("storageclasses").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("persistentvolumeclaims").verbs("*")

	rb.addRoleTemplate("View Volumes", "persistentvolumeclaims-view", "project", false, false, false).
		addRule().apiGroups("").resources("persistentvolumes").verbs("get", "list", "watch").
		addRule().apiGroups("storage.k8s.io").resources("storageclasses").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("persistentvolumeclaims").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Service Accounts", "serviceaccounts-manage", "project", false, false, false).
		addRule().apiGroups("").resources("serviceaccounts").verbs("*")

	rb.addRoleTemplate("View Service Accounts", "serviceaccounts-view", "project", false, false, false).
		addRule().apiGroups("").resources("serviceaccounts").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Project Members", "projectroletemplatebindings-manage", "project", false, false, false).
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("*")

	rb.addRoleTemplate("View Project Members", "projectroletemplatebindings-view", "project", false, false, false).
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Project Catalogs", "projectcatalogs-manage", "project", false, false, false).
		addRule().apiGroups("management.cattle.io").resources("projectcatalogs").verbs("*")

	rb.addRoleTemplate("View Project Catalogs", "projectcatalogs-view", "project", false, false, false).
		addRule().apiGroups("management.cattle.io").resources("projectcatalogs").verbs("get", "list", "watch")

	rb.addRoleTemplate("Project Monitoring View Role", "project-monitoring-readonly", "project", false, true, false).
		addRule().apiGroups("monitoring.cattle.io").resources("prometheus").verbs("view").
		setRoleTemplateNames("view")

	proxyNames := []string{
		"http:rancher-monitoring-prometheus:9090",
		"https:rancher-monitoring-prometheus:9090",
		"http:rancher-monitoring-alertmanager:9093",
		"https:rancher-monitoring-alertmanager:9093",
		"http:rancher-monitoring-grafana:80",
		"https:rancher-monitoring-grafana:80",
	}
	endpointNames := []string{
		"rancher-monitoring-prometheus",
		"rancher-monitoring-alertmanager",
		"rancher-monitoring-grafana",
	}

	rb.addRoleTemplate("View Monitoring", "monitoring-ui-view", "project", true, false, false).
		addExternalRule().apiGroups("").resources("services/proxy").verbs("get", "create").resourceNames(proxyNames...).
		addExternalRule().apiGroups("").resources("endpoints").verbs("list").resourceNames(endpointNames...)

	rb.addRoleTemplate("View Navlinks", "navlinks-view", "project", false, false, false).
		addRule().apiGroups("ui.cattle.io").resources("navlinks").verbs("get", "list", "watch")

	// Not specific to project or cluster
	// TODO When clusterevents has value, consider adding this back in
	//rb.addRoleTemplate("View Events", "events-view", "", true, false, false).
	//	addRule().apiGroups("").resources("events").verbs("get", "list", "watch").
	//	addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch")

	if err := rb.reconcileRoleTemplates(wrangler.Mgmt.RoleTemplate()); err != nil {
		return "", errors.Wrap(err, "problem reconciling role templates")
	}

	adminName, err := BootstrapAdmin(wrangler)
	if err != nil {
		return "", err
	}

	err = bootstrapDefaultRoles(management)
	if err != nil {
		return "", err
	}

	return adminName, nil
}

func addUserRules(role *roleBuilder) *roleBuilder {
	role.
		addRule().apiGroups("").resources("secrets").verbs("create").
		addRule().apiGroups("management.cattle.io").resources("principals", "roletemplates").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("preferences").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("settings").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("features").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("templates", "templateversions", "catalogs").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusters").verbs("create").
		addRule().apiGroups("management.cattle.io").resources("nodedrivers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("kontainerdrivers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("nodetemplates").verbs("create").
		addRule().apiGroups("management.cattle.io").resources("fleetworkspaces").verbs("create").
		addRule().apiGroups("management.cattle.io").resources("multiclusterapps", "globaldnses", "globaldnsproviders", "clustertemplaterevisions").verbs("create").
		addRule().apiGroups("management.cattle.io").resources("rkek8ssystemimages").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("rkek8sserviceoptions").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("rkeaddons").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("cisconfigs").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("cisbenchmarkversions").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("sourcecodecredentials").verbs("*").
		addRule().apiGroups("project.cattle.io").resources("sourcecoderepositories").verbs("*").
		addRule().apiGroups("provisioning.cattle.io").resources("clusters").verbs("create").
		addRule().apiGroups("rke-machine-config.cattle.io").resources("*").verbs("create").
		addRule().apiGroups("management.cattle.io").resources("rancherusernotifications").verbs("get", "list", "watch")

	return role
}

// BootstrapAdmin checks if the bootstrapAdminConfig exists, if it does this indicates rancher has
// already created the admin user and should not attempt it again. Otherwise attempt to create the admin.
func BootstrapAdmin(management *wrangler.Context) (string, error) {
	adminCreateLock.Lock()
	defer adminCreateLock.Unlock()

	if settings.NoDefaultAdmin.Get() == "true" {
		return "", nil
	}
	var adminName string

	set := labels.Set(defaultAdminLabel)
	admins, err := management.Mgmt.User().List(v1.ListOptions{LabelSelector: set.String()})
	if err != nil {
		return "", err
	}

	if len(admins.Items) > 0 {
		adminName = admins.Items[0].Name
	}

	if _, err := management.K8s.CoreV1().ConfigMaps(cattleNamespace).Get(context.TODO(), bootstrapAdminConfig, v1.GetOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			logrus.Warnf("Unable to determine if admin user already created: %v", err)
			return "", nil
		}
	} else {
		// config map already exists, nothing to do
		return adminName, nil
	}

	users, err := management.Mgmt.User().List(v1.ListOptions{})
	if err != nil {
		return "", err
	}

	if len(users.Items) == 0 {
		// Config map does not exist and no users, attempt to create the default admin user
		bootstrapPassword, bootstrapPasswordIsGenerated, err := GetBootstrapPassword(context.TODO(), management.K8s.CoreV1().Secrets(cattleNamespace))
		if err != nil {
			return "", errors.Wrap(err, "failed to retrieve bootstrap password")
		}

		bootstrapPasswordHash, _ := bcrypt.GenerateFromPassword([]byte(bootstrapPassword), bcrypt.DefaultCost)

		admin, err := management.Mgmt.User().Create(&v3.User{
			ObjectMeta: v1.ObjectMeta{
				GenerateName: "user-",
				Labels:       defaultAdminLabel,
			},
			DisplayName:        "Default Admin",
			Username:           "admin",
			Password:           string(bootstrapPasswordHash),
			MustChangePassword: bootstrapPasswordIsGenerated || bootstrapPassword == "admin",
		})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return "", errors.Wrap(err, "can not ensure admin user exists")
		}
		if err == nil {
			var serverURL string
			if settings.ServerURL.Get() != "" {
				serverURL = settings.ServerURL.Get()
			}
			if serverURL == "" {
				ip, err := net.ChooseHostInterface()
				if err == nil {
					serverURL = "https://" + ip.String()
				}
			}
			if serverURL == "" {
				serverURL = "https://" + "localhost"
			}

			logrus.Infof("")
			logrus.Infof("-----------------------------------------")
			logrus.Infof("Welcome to Rancher")
			if bootstrapPasswordIsGenerated {
				logrus.Infof("A bootstrap password has been generated for your admin user.")
				logrus.Infof("")
				logrus.Infof("Bootstrap Password: %s", bootstrapPassword)
				logrus.Infof("")
				logrus.Infof("Use %s/dashboard/?setup=%s to complete setup in the UI", serverURL, bootstrapPassword)
			} else {
				logrus.Infof("")
				logrus.Infof("Use %s/dashboard/ to complete setup in the UI", serverURL)
			}
			logrus.Infof("-----------------------------------------")
			logrus.Infof("")
		}
		adminName = admin.Name

		bindings, err := management.Mgmt.GlobalRoleBinding().List(v1.ListOptions{LabelSelector: set.String()})
		if err != nil {
			logrus.Warnf("Failed to create default admin global role binding: %v", err)
			bindings = &v3.GlobalRoleBindingList{}
		}
		if len(bindings.Items) == 0 {
			adminRole := "admin"
			if settings.RestrictedDefaultAdmin.Get() == "true" {
				adminRole = "restricted-admin"
			}
			_, err = management.Mgmt.GlobalRoleBinding().Create(
				&v3.GlobalRoleBinding{
					ObjectMeta: v1.ObjectMeta{
						GenerateName: "globalrolebinding-",
						Labels:       defaultAdminLabel,
					},
					UserName:       adminName,
					GlobalRoleName: adminRole,
				})
			if err != nil && !features.MCM.Enabled() {
				_, crbErr := management.RBAC.ClusterRoleBinding().Create(&rbacv1.ClusterRoleBinding{
					ObjectMeta: v1.ObjectMeta{
						GenerateName: "default-admin-",
						Labels:       defaultAdminLabel,
					},
					Subjects: []rbacv1.Subject{{
						Kind:     "User",
						APIGroup: rbacv1.GroupName,
						Name:     adminName,
					}},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.GroupName,
						Kind:     "ClusterRole",
						Name:     "cluster-admin",
					},
				})
				if crbErr != nil {
					logrus.Warnf("Failed to create default admin global role binding: %v", err)
				}
			} else if err != nil {
				logrus.Warnf("Failed to create default admin global role binding: %v", err)
			} else {
				logrus.Info("Created default admin user and binding")
			}
		}
	}

	adminConfigMap := corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      bootstrapAdminConfig,
			Namespace: cattleNamespace,
		},
	}

	_, err = management.K8s.CoreV1().ConfigMaps(cattleNamespace).Create(context.TODO(), &adminConfigMap, v1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			logrus.Warnf("Error creating admin config map: %v", err)
		}

	}
	return adminName, nil
}

// bootstrapDefaultRoles will set the default roles for user login, cluster create
// and project create. If the default roles already have the bootstrappedRole
// annotation this will be a no-op as this was done on a previous startup and will
// now respect the currently selected defaults.
func bootstrapDefaultRoles(management *config.ManagementContext) error {
	user, err := management.Management.GlobalRoles("").Get("user", v1.GetOptions{})
	if err != nil {
		return err
	}
	if _, ok := user.Annotations[bootstrappedRole]; !ok {
		copy := user.DeepCopy()
		copy.NewUserDefault = true
		if copy.Annotations == nil {
			copy.Annotations = make(map[string]string)
		}
		copy.Annotations[bootstrappedRole] = "true"

		_, err := management.Management.GlobalRoles("").Update(copy)
		if err != nil {
			return err
		}
	}

	clusterRole, err := management.Management.RoleTemplates("").Get("cluster-owner", v1.GetOptions{})
	if err != nil {
		return nil
	}
	if _, ok := clusterRole.Annotations[bootstrappedRole]; !ok {
		copy := clusterRole.DeepCopy()
		copy.ClusterCreatorDefault = true
		if copy.Annotations == nil {
			copy.Annotations = make(map[string]string)
		}
		copy.Annotations[bootstrappedRole] = "true"

		_, err := management.Management.RoleTemplates("").Update(copy)
		if err != nil {
			return err
		}
	}

	projectRole, err := management.Management.RoleTemplates("").Get("project-owner", v1.GetOptions{})
	if err != nil {
		return nil
	}
	if _, ok := projectRole.Annotations[bootstrappedRole]; !ok {
		copy := projectRole.DeepCopy()
		copy.ProjectCreatorDefault = true
		if copy.Annotations == nil {
			copy.Annotations = make(map[string]string)
		}
		copy.Annotations[bootstrappedRole] = "true"

		_, err := management.Management.RoleTemplates("").Update(copy)
		if err != nil {
			return err
		}
	}

	return nil
}

func addClusterRoleForNamespacedCRDs(management *config.ManagementContext) error {
	var returnErr error
	// If adding Rules for new CRDs to the below ClusterRole, make sure to add them in a sorted order
	// ClusterCRDsClusterRole is a CR containing rules for granting restricted-admins access to all CRDs that can be created in a v3.Cluster's namespace
	cr := rbacv1.ClusterRole{
		ObjectMeta: v1.ObjectMeta{
			Name: rbac.ClusterCRDsClusterRole,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"management.cattle.io"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		},
	}
	if err := createOrUpdateClusterRole(management, cr); err != nil {
		returnErr = multierror.Append(returnErr, err)
	}

	// ProjectCRDsClusterRole is a CR containing rules for granting restricted-admins access to all CRDs that can be created in a
	// v3.Cluster and v3.Project's namespace
	cr = rbacv1.ClusterRole{
		ObjectMeta: v1.ObjectMeta{
			Name: rbac.ProjectCRDsClusterRole,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"management.cattle.io"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{"project.cattle.io"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		},
	}
	if err := createOrUpdateClusterRole(management, cr); err != nil {
		returnErr = multierror.Append(returnErr, err)
	}
	return returnErr
}

func createOrUpdateClusterRole(management *config.ManagementContext, cr rbacv1.ClusterRole) error {
	for _, rule := range cr.Rules {
		sort.Slice(rule.APIGroups, func(i, j int) bool { return rule.APIGroups[i] < rule.APIGroups[j] })
	}
	sort.Slice(cr.Rules, func(i, j int) bool {
		return cr.Rules[i].APIGroups[0] < cr.Rules[j].APIGroups[0]
	})
	_, err := management.RBAC.ClusterRoles("").Create(&cr)
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			existingCR, err := management.RBAC.ClusterRoles("").Get(cr.Name, v1.GetOptions{})
			if err != nil {
				return err
			}
			if reflect.DeepEqual(cr.Rules, existingCR.Rules) {
				return nil
			}
			existingCR.Rules = cr.Rules
			_, err = management.RBAC.ClusterRoles("").Update(existingCR)
			return err
		})
		return err
	}
	return nil
}
