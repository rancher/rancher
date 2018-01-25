package app

import (
	"context"
	"crypto/x509"
	"log"

	"time"

	"bytes"
	"encoding/pem"

	"github.com/pkg/errors"
	managementController "github.com/rancher/cluster-controller/controller"
	"github.com/rancher/rancher/server"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/cert"
)

type Config struct {
	HTTPOnly          bool
	ACMEDomains       []string
	KubeConfig        string
	HTTPListenPort    int
	HTTPSListenPort   int
	InteralListenPort int
	K8sMode           string
	AddLocal          bool
	Debug             bool
	ListenConfig      *v3.ListenConfig
}

var defaultAdminLabel = map[string]string{"authz.management.cattle.io/bootstrapping": "admin-user"}

func Run(ctx context.Context, kubeConfig rest.Config, cfg *Config) error {
	management, err := config.NewManagementContext(kubeConfig)
	if err != nil {
		return err
	}
	management.LocalConfig = &kubeConfig

	if err := ReadTLSConfig(cfg); err != nil {
		return err
	}

	for {
		_, err := management.K8sClient.Discovery().ServerVersion()
		if err == nil {
			break
		}
		logrus.Infof("Waiting for server to become available: %v", err)
		time.Sleep(2 * time.Second)
	}

	if err := server.New(ctx, cfg.HTTPListenPort, cfg.HTTPSListenPort, management); err != nil {
		return err
	}

	managementController.Register(ctx, management)
	if err := management.Start(ctx); err != nil {
		return err
	}

	if err := addData(management, *cfg); err != nil {
		return err
	}

	<-ctx.Done()
	if ctx.Err() != nil {
		log.Fatal(ctx.Err())
	}

	return ctx.Err()
}

func addData(management *config.ManagementContext, cfg Config) error {
	if err := addListenConfig(management, cfg); err != nil {
		return err
	}

	if err := addRoles(management, cfg.AddLocal); err != nil {
		return err
	}

	return addMachineDrivers(management)
}

func addListenConfig(management *config.ManagementContext, cfg Config) error {
	existing, err := management.Management.ListenConfigs("").Get(cfg.ListenConfig.Name, v1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if apierrors.IsNotFound(err) {
		existing = nil
	}

	if existing != nil {
		if cfg.ListenConfig.CACerts == "" {
			cfg.ListenConfig.CACerts = existing.CACerts
		}
		if cfg.ListenConfig.Key == "" {
			cfg.ListenConfig.Key = existing.Key
		}
		if cfg.ListenConfig.Cert == "" {
			cfg.ListenConfig.Cert = existing.Cert
		}
		if cfg.ListenConfig.CAKey == "" {
			cfg.ListenConfig.CAKey = existing.CAKey
		}
		if cfg.ListenConfig.CACert == "" {
			cfg.ListenConfig.CACert = existing.CACert
		}
		if len(cfg.ListenConfig.KnownIPs) == 0 {
			cfg.ListenConfig.KnownIPs = existing.KnownIPs
		}
	}

	if (cfg.ListenConfig.Key == "" || cfg.ListenConfig.Cert == "") && cfg.ListenConfig.CACert == "" && cfg.ListenConfig.Mode != "acme" {
		caKey, err := cert.NewPrivateKey()
		if err != nil {
			return err
		}

		caCert, err := cert.NewSelfSignedCACert(cert.Config{
			CommonName:   "cattle-ca",
			Organization: []string{"the-rancher"},
		}, caKey)
		if err != nil {
			return err
		}

		caCertBuffer := bytes.Buffer{}
		if err := pem.Encode(&caCertBuffer, &pem.Block{
			Type:  cert.CertificateBlockType,
			Bytes: caCert.Raw,
		}); err != nil {
			return err
		}

		caKeyBuffer := bytes.Buffer{}
		if err := pem.Encode(&caKeyBuffer, &pem.Block{
			Type:  cert.RSAPrivateKeyBlockType,
			Bytes: x509.MarshalPKCS1PrivateKey(caKey),
		}); err != nil {
			return err
		}

		cfg.ListenConfig.CACert = string(caCertBuffer.Bytes())
		cfg.ListenConfig.CACerts = cfg.ListenConfig.CACert
		cfg.ListenConfig.CAKey = string(caKeyBuffer.Bytes())
	}

	if cfg.ListenConfig.Mode == "acme" {
		cfg.ListenConfig.CACerts = ""
	}

	if existing == nil {
		_, err := management.Management.ListenConfigs("").Create(cfg.ListenConfig)
		return err
	}

	cfg.ListenConfig.ResourceVersion = existing.ResourceVersion
	_, err = management.Management.ListenConfigs("").Update(cfg.ListenConfig)
	return err
}

func addRoles(management *config.ManagementContext, local bool) error {
	rb := newRoleBuilder()

	rb.addRole("Create Clusters", "create-clusters").addRule().apiGroups("management.cattle.io").resources("clusters").verbs("create")
	rb.addRole("Manage All Clusters", "manage-clusters").addRule().apiGroups("management.cattle.io").resources("clusters").verbs("*")
	rb.addRole("Manage Node Drivers", "manage-node-drivers").addRule().apiGroups("management.cattle.io").resources("machinedrivers").verbs("*")
	rb.addRole("Manage Catalogs", "manage-catalogs").addRule().apiGroups("management.cattle.io").resources("catalogs", "templates", "templateversions").verbs("*")
	rb.addRole("Use Catalog Templates", "use-catalogs").addRule().apiGroups("management.cattle.io").resources("templates", "templateversions").verbs("get", "list", "watch")
	rb.addRole("Manage Users", "manage-users").addRule().apiGroups("management.cattle.io").resources("users", "globalroles", "globalrolebindings").verbs("*")
	rb.addRole("Manage Roles", "manage-roles").addRule().apiGroups("management.cattle.io").resources("roletemplates").verbs("*")
	rb.addRole("Manage Authentication", "manage-authn").addRule().apiGroups("management.cattle.io").resources("authconfigs").verbs("get", "list", "watch", "update")
	rb.addRole("Manage Node Templates", "manage-node-templates").addRule().apiGroups("management.cattle.io").resources("machinetemplates").verbs("*")
	rb.addRole("Use Node Templates", "use-node-templates").addRule().apiGroups("management.cattle.io").resources("machinedrivers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("machinetemplates").verbs("*")
	rb.addRole("Manage Settings", "manage-settings").addRule().apiGroups("management.cattle.io").resources("settings").verbs("*")

	rb.addRole("Admin", "admin").addRule().apiGroups("*").resources("*").verbs("*").
		addRule().apiGroups().nonResourceURLs("*").verbs("*")

	rb.addRole("User", "user").addRule().apiGroups("management.cattle.io").resources("principals", "roletemplates").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("users").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("preferences").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("clusters").verbs("create").
		addRule().apiGroups("management.cattle.io").resources("templates", "templateversions").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("machinedrivers").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("machinetemplates").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("settings").verbs("get", "list", "watch")

	rb.addRole("User Base", "user-base").addRule().apiGroups("management.cattle.io").resources("principals", "roletemplates").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("users").verbs("get", "list", "watch").
		addRule().apiGroups("management.cattle.io").resources("preferences").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("settings").verbs("get", "list", "watch")

	// TODO user should be dynamically authorized to only see herself
	// TODO Need "self-service" for machinetemplates such that a user can create them, but only RUD their own
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
		addRule().apiGroups("management.cattle.io").resources("machines").verbs("get", "list", "watch").
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
		addRule().apiGroups("management.cattle.io").resources("machines").verbs("*").
		addRule().apiGroups("*").resources("nodes").verbs("*")

	rb.addRoleTemplate("View Nodes", "view-nodes", "cluster", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("machines").verbs("get", "list", "watch").
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

func addMachineDrivers(management *config.ManagementContext) error {
	if err := addMachineDriver("amazonec2", "local://", "", true, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("azure", "local://", "", true, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("digitalocean", "local://", "", true, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("exoscale", "local://", "", false, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("generic", "local://", "", false, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("google", "local://", "", false, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("openstack", "local://", "", false, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("otc", "https://obs.otc.t-systems.com/dockermachinedriver/docker-machine-driver-otc",
		"e98f246f625ca46f5e037dc29bdf00fe", false, false, management); err != nil {
		return err
	}
	if err := addMachineDriver("packet", "https://github.com/packethost/docker-machine-driver-packet/releases/download/v0.1.2/docker-machine-driver-packet_linux-amd64.zip",
		"cd610cd7d962dfdf88a811ec026bcdcf", true, false, management); err != nil {
		return err
	}
	if err := addMachineDriver("rackspace", "local://", "", false, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("softlayer", "local://", "", false, true, management); err != nil {
		return err
	}
	if err := addMachineDriver("vmwarevcloudair", "local://", "", false, true, management); err != nil {
		return err
	}

	return addMachineDriver("vmwarevsphere", "local://", "", true, true, management)
}

func addMachineDriver(name, url, checksum string, active, builtin bool, management *config.ManagementContext) error {
	lister := management.Management.MachineDrivers("").Controller().Lister()
	cli := management.Management.MachineDrivers("")
	m, _ := lister.Get("", name)
	if m != nil {
		if m.Spec.Builtin != builtin || m.Spec.URL != url || m.Spec.Checksum != checksum || m.Spec.DisplayName != name {
			logrus.Infof("Updating machine driver %v", name)
			m.Spec.Builtin = builtin
			m.Spec.URL = url
			m.Spec.Checksum = checksum
			m.Spec.DisplayName = name
			_, err := cli.Update(m)
			return err
		}
		return nil
	}

	logrus.Infof("Creating machine driver %v", name)
	_, err := cli.Create(&v3.MachineDriver{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
		Spec: v3.MachineDriverSpec{
			Active:      active,
			Builtin:     builtin,
			URL:         url,
			DisplayName: name,
			Checksum:    checksum,
		},
	})

	return err
}
