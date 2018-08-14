package app

import (
	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"golang.org/x/crypto/bcrypt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	bootstrappedRole = "authz.management.cattle.io/bootstrapped-role"
)

var defaultAdminLabel = map[string]string{"authz.management.cattle.io/bootstrapping": "admin-user"}

func addRoles(management *config.ManagementContext) (string, error) {
	rb := newRoleBuilder()

	rb.addRole("Create Clusters", "clusters-create").addRule().apiGroups("management.cattle.io").resources("clusters").verbs("create").
		addRule().apiGroups("management.cattle.io").resources("templates", "templateversions").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("nodedrivers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("podsecuritypolicytemplates").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("nodetemplates").verbs("*")
	rb.addRole("Manage Node Drivers", "nodedrivers-manage").addRule().apiGroups("management.cattle.io").resources("nodedrivers").verbs("*")
	rb.addRole("Manage Catalogs", "catalogs-manage").addRule().apiGroups("management.cattle.io").resources("catalogs", "templates", "templateversions").verbs("*")
	rb.addRole("Use Catalog Templates", "catalogs-use").addRule().apiGroups("management.cattle.io").resources("templates", "templateversions").verbs("get", "list", "watch")
	rb.addRole("Manage Users", "users-manage").addRule().apiGroups("management.cattle.io").resources("users", "globalrolebindings").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("globalroles").verbs("get", "list", "watch")
	rb.addRole("Manage Roles", "roles-manage").addRule().apiGroups("management.cattle.io").resources("roletemplates").verbs("*")
	rb.addRole("Manage Authentication", "authn-manage").addRule().apiGroups("management.cattle.io").resources("authconfigs").verbs("get", "list", "watch", "update")
	rb.addRole("Manage Settings", "settings-manage").addRule().apiGroups("management.cattle.io").resources("settings").verbs("*")
	rb.addRole("Manage PodSecurityPolicy Templates", "podsecuritypolicytemplates-manage").addRule().apiGroups("management.cattle.io").resources("podsecuritypolicytemplates").verbs("*")
	rb.addRole("Manage ResourceQuota Templates", "resourcequotatemplates-manage").addRule().apiGroups("management.cattle.io").resources("resourcequotatemplates").verbs("*")

	rb.addRole("Admin", "admin").addRule().apiGroups("*").resources("*").verbs("*").
		addRule().apiGroups().nonResourceURLs("*").verbs("*")

	rb.addRole("User", "user").addRule().apiGroups("management.cattle.io").resources("principals", "roletemplates").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("preferences").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("settings").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusters").verbs("create").
		addRule().apiGroups("management.cattle.io").resources("templates", "templateversions").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("nodedrivers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("podsecuritypolicytemplates").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("nodetemplates").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("sourcecodecredentials").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("sourcecoderepositories").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("resourcequotatemplates").verbs("get", "list", "watch")

	rb.addRole("User Base", "user-base").addRule().apiGroups("management.cattle.io").resources("preferences").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("settings").verbs("get", "list", "watch")

	// TODO user should be dynamically authorized to only see herself
	// TODO Need "self-service" for nodetemplates such that a user can create them, but only RUD their own
	// TODO enable when groups are "in". they need to be self-service

	if err := rb.reconcileGlobalRoles(management); err != nil {
		return "", errors.Wrap(err, "problem reconciling global roles")
	}

	// RoleTemplates to be used inside of clusters
	rb = newRoleBuilder()

	// K8s default roles
	rb.addRoleTemplate("Kubernetes cluster-admin", "cluster-admin", "cluster", true, true, true)
	rb.addRoleTemplate("Kubernetes admin", "admin", "project", true, true, true)
	rb.addRoleTemplate("Kubernetes edit", "edit", "project", true, true, true)
	rb.addRoleTemplate("Kubernetes view", "view", "project", true, true, true)

	// Cluster roles
	rb.addRoleTemplate("Cluster Owner", "cluster-owner", "cluster", true, false, false).
		addRule().apiGroups("*").resources("*").verbs("*").
		addRule().apiGroups().nonResourceURLs("*").verbs("*")

	rb.addRoleTemplate("Cluster Member", "cluster-member", "cluster", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("clusterroletemplatebindings").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projects").verbs("create").
		addRule().apiGroups("management.cattle.io").resources("nodes", "nodepools").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("nodes").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("persistentvolumes").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("storageclasses").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusterpipelines").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusterloggings").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusteralerts").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("notifiers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("resourcequotatemplates").verbs("*")

	rb.addRoleTemplate("Create Projects", "projects-create", "cluster", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projects").verbs("create")

	rb.addRoleTemplate("View All Projects", "projects-view", "cluster", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projects").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("apps").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("namespaces").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("persistentvolumes").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("storageclasses").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("persistentvolumeclaims").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("resourcequotatemplates").verbs("get", "list", "watch").
		setRoleTemplateNames("view")

	rb.addRoleTemplate("Manage Nodes", "nodes-manage", "cluster", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("nodes", "nodepools").verbs("*").
		addRule().apiGroups("*").resources("nodes").verbs("*")

	rb.addRoleTemplate("View Nodes", "nodes-view", "cluster", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("nodes", "nodepools").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("nodes").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Storage", "storage-manage", "cluster", true, false, false).
		addRule().apiGroups("*").resources("persistentvolumes").verbs("*").
		addRule().apiGroups("*").resources("storageclasses").verbs("*").
		addRule().apiGroups("*").resources("persistentvolumeclaims").verbs("*")

	rb.addRoleTemplate("Manage Cluster Members", "clusterroletemplatebindings-manage", "cluster", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("clusterroletemplatebindings").verbs("*")

	rb.addRoleTemplate("View Cluster Members", "clusterroletemplatebindings-view", "cluster", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("clusterroletemplatebindings").verbs("get", "list", "watch")

	// Project roles
	rb.addRoleTemplate("Project Owner", "project-owner", "project", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("*").
		addRule().apiGroups("project.cattle.io").resources("apps").verbs("*").
		addRule().apiGroups("").resources("namespaces").verbs("create").
		addRule().apiGroups("*").resources("persistentvolumes").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("storageclasses").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("persistentvolumeclaims").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusterpipelines").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("pipelines").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("pipelineexecutions").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("pipelineexecutionlogs").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("notifiers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectalerts").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("projectloggings").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("resourcequotatemplates").verbs("get", "list", "watch").
		setRoleTemplateNames("admin")

	rb.addRoleTemplate("Project Member", "project-member", "project", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("apps").verbs("*").
		addRule().apiGroups("").resources("namespaces").verbs("create").
		addRule().apiGroups("*").resources("persistentvolumes").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("storageclasses").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("persistentvolumeclaims").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusterpipelines").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("pipelines").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("pipelineexecutions").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("pipelineexecutionlogs").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("notifiers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectalerts").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("projectloggings").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("resourcequotatemplates").verbs("get", "list", "watch").
		setRoleTemplateNames("edit")

	rb.addRoleTemplate("Read-only", "read-only", "project", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("apps").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("persistentvolumes").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("storageclasses").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("persistentvolumeclaims").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusterpipelines").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("pipelines").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("pipelineexecutions").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("pipelineexecutionlogs").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("notifiers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectalerts").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("projectloggings").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("resourcequotatemplates").verbs("get", "list", "watch").
		setRoleTemplateNames("view")

	rb.addRoleTemplate("Create Namespaces", "create-ns", "project", true, false, false).
		addRule().apiGroups("").resources("namespaces").verbs("create")

	rb.addRoleTemplate("Manage Workloads", "workloads-manage", "project", true, false, false).
		addRule().apiGroups("*").resources("pods", "pods/attach", "pods/exec", "pods/portforward", "pods/proxy", "replicationcontrollers",
		"replicationcontrollers/scale", "daemonsets", "deployments", "deployments/rollback", "deployments/scale", "replicasets",
		"replicasets/scale", "statefulsets", "cronjobs", "jobs", "daemonsets", "deployments", "deployments/rollback", "deployments/scale",
		"replicasets", "replicasets/scale", "replicationcontrollers/scale", "horizontalpodautoscalers").verbs("*").
		addRule().apiGroups("*").resources("limitranges", "pods/log", "pods/status", "replicationcontrollers/status", "resourcequotas", "resourcequotas/status", "bindings").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("apps").verbs("*")

	rb.addRoleTemplate("View Workloads", "workloads-view", "project", true, false, false).
		addRule().apiGroups("*").resources("pods", "pods/attach", "pods/exec", "pods/portforward", "pods/proxy", "replicationcontrollers",
		"replicationcontrollers/scale", "daemonsets", "deployments", "deployments/rollback", "deployments/scale", "replicasets",
		"replicasets/scale", "statefulsets", "cronjobs", "jobs", "daemonsets", "deployments", "deployments/rollback", "deployments/scale",
		"replicasets", "replicasets/scale", "replicationcontrollers/scale", "horizontalpodautoscalers").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("limitranges", "pods/log", "pods/status", "replicationcontrollers/status", "resourcequotas", "resourcequotas/status", "bindings").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("apps").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Ingress", "ingress-manage", "project", true, false, false).
		addRule().apiGroups("*").resources("ingresses").verbs("*")

	rb.addRoleTemplate("View Ingress", "ingress-view", "project", true, false, false).
		addRule().apiGroups("*").resources("ingresses").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Services", "services-manage", "project", true, false, false).
		addRule().apiGroups("*").resources("services", "endpoints").verbs("*")

	rb.addRoleTemplate("View Services", "services-view", "project", true, false, false).
		addRule().apiGroups("*").resources("services", "endpoints").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Secrets", "secrets-manage", "project", true, false, false).
		addRule().apiGroups("*").resources("secrets").verbs("*")

	rb.addRoleTemplate("View Secrets", "secrets-view", "project", true, false, false).
		addRule().apiGroups("*").resources("secrets").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Config Maps", "configmaps-manage", "project", true, false, false).
		addRule().apiGroups("*").resources("configmaps").verbs("*")

	rb.addRoleTemplate("View Config Maps", "configmaps-view", "project", true, false, false).
		addRule().apiGroups("*").resources("configmaps").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Volumes", "persistentvolumeclaims-manage", "project", true, false, false).
		addRule().apiGroups("*").resources("persistentvolumes").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("storageclasses").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("persistentvolumeclaims").verbs("*")

	rb.addRoleTemplate("View Volumes", "persistentvolumeclaims-view", "project", true, false, false).
		addRule().apiGroups("*").resources("persistentvolumes").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("storageclasses").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("persistentvolumeclaims").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Service Accounts", "serviceaccounts-manage", "project", true, false, false).
		addRule().apiGroups("*").resources("serviceaccounts").verbs("*")

	rb.addRoleTemplate("View Service Accounts", "serviceaccounts-view", "project", true, false, false).
		addRule().apiGroups("*").resources("serviceaccounts").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Project Members", "projectroletemplatebindings-manage", "project", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("*")

	rb.addRoleTemplate("View Project Members", "projectroletemplatebindings-view", "project", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("get", "list", "watch")

	// Not specific to project or cluster
	// TODO When clusterevents has value, consider adding this back in
	//rb.addRoleTemplate("View Events", "events-view", "", true, false, false).
	//	addRule().apiGroups("*").resources("events").verbs("get", "list", "watch").
	//	addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch")

	if err := rb.reconcileRoleTemplates(management); err != nil {
		return "", errors.Wrap(err, "problem reconciling role templates")
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)

	set := labels.Set(defaultAdminLabel)
	admins, err := management.Management.Users("").List(v1.ListOptions{LabelSelector: set.String()})
	if err != nil {
		return "", err
	}

	// TODO This logic is going to be a problem in an HA setup because a race will cause more than one admin user to be created
	var admin *v3.User
	if len(admins.Items) == 0 {
		admin, err = management.Management.Users("").Create(&v3.User{
			ObjectMeta: v1.ObjectMeta{
				GenerateName: "user-",
				Labels:       defaultAdminLabel,
			},
			DisplayName:        "Default Admin",
			Username:           "admin",
			Password:           string(hash),
			MustChangePassword: true,
		})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return "", errors.Wrap(err, "can not ensure admin user exists")
		}

	} else {
		admin = &admins.Items[0]
	}

	bindings, err := management.Management.GlobalRoleBindings("").List(v1.ListOptions{LabelSelector: set.String()})
	if err != nil {
		return "", err
	}
	if len(bindings.Items) == 0 {
		management.Management.GlobalRoleBindings("").Create(
			&v3.GlobalRoleBinding{
				ObjectMeta: v1.ObjectMeta{
					GenerateName: "globalrolebinding-",
					Labels:       defaultAdminLabel,
				},
				UserName:       admin.Name,
				GlobalRoleName: "admin",
			})
	}

	err = bootstrapDefaultRoles(management)
	if err != nil {
		return "", err
	}

	return admin.Name, nil
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
