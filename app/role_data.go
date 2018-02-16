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

func addRoles(management *config.ManagementContext, local bool) error {
	rb := newRoleBuilder()

	rb.addRole("Create Clusters", "create-clusters").addRule().apiGroups("management.cattle.io").resources("clusters").verbs("create")
	rb.addRole("Manage All Clusters", "manage-clusters").addRule().apiGroups("management.cattle.io").resources("clusters").verbs("*")
	rb.addRole("Manage Node Drivers", "manage-node-drivers").addRule().apiGroups("management.cattle.io").resources("nodedrivers").verbs("*")
	rb.addRole("Manage Catalogs", "manage-catalogs").addRule().apiGroups("management.cattle.io").resources("catalogs", "templates", "templateversions").verbs("*")
	rb.addRole("Use Catalog Templates", "use-catalogs").addRule().apiGroups("management.cattle.io").resources("templates", "templateversions").verbs("get", "list", "watch")
	rb.addRole("Manage Users", "manage-users").addRule().apiGroups("management.cattle.io").resources("users", "globalroles", "globalrolebindings").verbs("*")
	rb.addRole("Manage Roles", "manage-roles").addRule().apiGroups("management.cattle.io").resources("roletemplates").verbs("*")
	rb.addRole("Manage Authentication", "manage-authn").addRule().apiGroups("management.cattle.io").resources("authconfigs").verbs("get", "list", "watch", "update")
	rb.addRole("Manage Node Templates", "manage-node-templates").addRule().apiGroups("management.cattle.io").resources("nodetemplates").verbs("*")
	rb.addRole("Use Node Templates", "use-node-templates").addRule().apiGroups("management.cattle.io").resources("nodedrivers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("nodetemplates").verbs("*")
	rb.addRole("Manage Settings", "manage-settings").addRule().apiGroups("management.cattle.io").resources("settings").verbs("*")

	rb.addRole("Admin", "admin").addRule().apiGroups("*").resources("*").verbs("*").
		addRule().apiGroups().nonResourceURLs("*").verbs("*")

	rb.addRole("User", "user").addRule().apiGroups("management.cattle.io").resources("principals", "roletemplates").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("users").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("preferences").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("clusters").verbs("create").
		addRule().apiGroups("management.cattle.io").resources("templates", "templateversions").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("nodedrivers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("nodetemplates").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("settings").verbs("get", "list", "watch")

	rb.addRole("User Base", "user-base").addRule().apiGroups("management.cattle.io").resources("principals", "roletemplates").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("users").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("preferences").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("settings").verbs("get", "list", "watch")

	// TODO user should be dynamically authorized to only see herself
	// TODO Need "self-service" for nodetemplates such that a user can create them, but only RUD their own
	// TODO enable when groups are "in". they need to be self-service

	if err := rb.reconcileGlobalRoles(management); err != nil {
		return errors.Wrap(err, "problem reconciling globl roles")
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
		addRule().apiGroups("management.cattle.io").resources("nodes").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("nodes").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("persistentvolumes").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Projects", "manage-projects", "cluster", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projects").verbs("*")

	rb.addRoleTemplate("Create Projects", "create-projects", "cluster", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projects").verbs("create")

	rb.addRoleTemplate("View Projects", "view-projects", "cluster", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projects").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Nodes", "manage-nodes", "cluster", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("nodes").verbs("*").
		addRule().apiGroups("*").resources("nodes").verbs("*")

	rb.addRoleTemplate("View Nodes", "view-nodes", "cluster", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("nodes").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("nodes").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Volumes", "manage-volumes", "cluster", true, false, false).
		addRule().apiGroups("*").resources("persistentvolumes").verbs("*")

	rb.addRoleTemplate("View Volumes", "view-volumes", "cluster", true, false, false).
		addRule().apiGroups("*").resources("persistentvolumes").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Cluster Members", "manage-cluster-members", "cluster", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("clusterroletemplatebindings").verbs("*")

	rb.addRoleTemplate("View Cluster Members", "view-cluster-members", "cluster", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("clusterroletemplatebindings").verbs("get", "list", "watch")

	// Project roles
	rb.addRoleTemplate("Project Owner", "project-owner", "project", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("*").
		addRule().apiGroups("project.cattle.io").resources("worklods").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch").
		addRule().apiGroups("").resources("namespaces").verbs("create").
		setRoleTemplateNames("admin")

	rb.addRoleTemplate("Project Member", "project-member", "project", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("worklods").verbs("*").
		addRule().apiGroups("").resources("namespaces").verbs("create").
		addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch").
		setRoleTemplateNames("edit")

	rb.addRoleTemplate("Read-only", "read-only", "project", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("get", "list", "watch").
		addRule().apiGroups("project.cattle.io").resources("worklods").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch").
		setRoleTemplateNames("view")

	rb.addRoleTemplate("Create Namespaces", "create-ns", "project", true, false, false).
		addRule().apiGroups("").resources("namespaces").verbs("create")

	rb.addRoleTemplate("Manage Workloads", "manage-workloads", "project", true, false, false).
		addRule().apiGroups("*").resources("pods", "pods/attach", "pods/exec", "pods/portforward", "pods/proxy", "replicationcontrollers",
		"replicationcontrollers/scale", "daemonsets", "deployments", "deployments/rollback", "deployments/scale", "replicasets",
		"replicasets/scale", "statefulsets", "cronjobs", "jobs", "daemonsets", "deployments", "deployments/rollback", "deployments/scale",
		"replicasets", "replicasets/scale", "replicationcontrollers/scale", "horizontalpodautoscalers").verbs("*").
		addRule().apiGroups("*").resources("limitranges", "pods/log", "pods/status", "replicationcontrollers/status", "resourcequotas", "resourcequotas/status", "bindings").verbs("get", "list", "watch")

	rb.addRoleTemplate("View Workloads", "view-workloads", "project", true, false, false).
		addRule().apiGroups("*").resources("pods", "pods/attach", "pods/exec", "pods/portforward", "pods/proxy", "replicationcontrollers",
		"replicationcontrollers/scale", "daemonsets", "deployments", "deployments/rollback", "deployments/scale", "replicasets",
		"replicasets/scale", "statefulsets", "cronjobs", "jobs", "daemonsets", "deployments", "deployments/rollback", "deployments/scale",
		"replicasets", "replicasets/scale", "replicationcontrollers/scale", "horizontalpodautoscalers").verbs("get", "list", "watch").
		addRule().apiGroups("*").resources("limitranges", "pods/log", "pods/status", "replicationcontrollers/status", "resourcequotas", "resourcequotas/status", "bindings").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Ingress", "manage-ingress", "project", true, false, false).
		addRule().apiGroups("*").resources("ingresses").verbs("*")

	rb.addRoleTemplate("View Ingress", "view-ingress", "project", true, false, false).
		addRule().apiGroups("*").resources("ingresses").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Service", "manage-service", "project", true, false, false).
		addRule().apiGroups("*").resources("services", "endpoints").verbs("*")

	rb.addRoleTemplate("View Services", "view-services", "project", true, false, false).
		addRule().apiGroups("*").resources("services", "endpoints").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Secrets", "manage-secrets", "project", true, false, false).
		addRule().apiGroups("*").resources("secrets").verbs("*")

	rb.addRoleTemplate("View Secrets", "view-secrets", "project", true, false, false).
		addRule().apiGroups("*").resources("secrets").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Config Maps", "manage-configmaps", "project", true, false, false).
		addRule().apiGroups("*").resources("configmaps").verbs("*")

	rb.addRoleTemplate("View Config Maps", "view-configmaps", "project", true, false, false).
		addRule().apiGroups("*").resources("configmaps").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Volumes", "manage-persistentvolumeclaims", "project", true, false, false).
		addRule().apiGroups("*").resources("persistentvolumeclaims").verbs("*")

	rb.addRoleTemplate("View Volumes", "view-persistentvolumeclaims", "project", true, false, false).
		addRule().apiGroups("*").resources("persistentvolumeclaims").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Service Accounts", "manage-serviceaccounts", "project", true, false, false).
		addRule().apiGroups("*").resources("serviceaccounts").verbs("*")

	rb.addRoleTemplate("View Service Accounts", "view-serviceaccounts", "project", true, false, false).
		addRule().apiGroups("*").resources("serviceaccounts").verbs("get", "list", "watch")

	rb.addRoleTemplate("Manage Project Members", "manage-project-members", "project", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("*")

	rb.addRoleTemplate("View Project Members", "view-project-members", "project", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("get", "list", "watch")

	// Not specific to project or cluster
	// TODO When clusterevents is replaced with events, remove clusterevents
	rb.addRoleTemplate("View Events", "view-events", "", true, false, false).
		addRule().apiGroups("*").resources("events").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("clusterevents").verbs("get", "list", "watch")

	if err := rb.reconcileRoleTemplates(management); err != nil {
		return errors.Wrap(err, "problem reconciling role templates")
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)

	set := labels.Set(defaultAdminLabel)
	admins, err := management.Management.Users("").List(v1.ListOptions{LabelSelector: set.String()})
	if err != nil {
		return err
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
			return errors.Wrap(err, "can not ensure admin user exists")
		}

	} else {
		admin = &admins.Items[0]
	}

	if len(admin.PrincipalIDs) == 0 {
		admin.PrincipalIDs = []string{"local://" + admin.Name}
		management.Management.Users("").Update(admin)
	}

	bindings, err := management.Management.GlobalRoleBindings("").List(v1.ListOptions{LabelSelector: set.String()})
	if err != nil {
		return err
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

	if local {
		// TODO If user delets the local cluster, this will recreate it on restart. Need to fix that
		management.Management.Clusters("").Create(&v3.Cluster{
			ObjectMeta: v1.ObjectMeta{
				Name: "local",
				Annotations: map[string]string{
					"field.cattle.io/creatorId": admin.Name,
				},
			},
			Spec: v3.ClusterSpec{
				DisplayName: "local",
				Internal:    true,
			},
		})
	}

	return nil
}
