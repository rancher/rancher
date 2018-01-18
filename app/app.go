package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"time"

	"github.com/pkg/errors"
	managementController "github.com/rancher/cluster-controller/controller"
	"github.com/rancher/rancher/server"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func Run(ctx context.Context, kubeConfig rest.Config, listenPort int, local bool) error {
	management, err := config.NewManagementContext(kubeConfig)
	if err != nil {
		return err
	}
	management.LocalConfig = &kubeConfig

	for {
		_, err := management.K8sClient.Discovery().ServerVersion()
		if err == nil {
			break
		}
		logrus.Infof("Waiting for server to become available: %v", err)
		time.Sleep(2 * time.Second)
	}

	handler, err := server.New(context.Background(), management)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		if ctx.Err() != nil {
			log.Fatal(ctx.Err())
		}
		os.Exit(1)
	}()

	managementController.Register(ctx, management)

	management.Start(ctx)

	if err := addData(management, local); err != nil {
		return err
	}

	fmt.Printf("Listening on 0.0.0.0:%d\n", listenPort)
	return http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", listenPort), handler)
}

func addData(management *config.ManagementContext, local bool) error {
	if local {
		management.Management.Clusters("").Create(&v3.Cluster{
			ObjectMeta: v1.ObjectMeta{
				Name: "local",
				Annotations: map[string]string{
					"field.cattle.io/creatorId": "admin",
				},
			},
			Spec: v3.ClusterSpec{
				DisplayName: "local",
				Internal:    true,
			},
		})
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	admin, err := management.Management.Users("").Create(&v3.User{
		ObjectMeta: v1.ObjectMeta{
			Name: "admin",
		},
		DisplayName:        "Default Admin",
		Username:           "admin",
		Password:           string(hash),
		MustChangePassword: true,
	})
	if err != nil {
		admin, _ = management.Management.Users("").Get("admin", v1.GetOptions{})
	}
	if len(admin.PrincipalIDs) == 0 {
		admin.PrincipalIDs = []string{"local://" + admin.Name}
		management.Management.Users("").Update(admin)
	}

	rb := newRoleBuilder()

	rb.addRole("Create Clusters", "create-clusters").addRule().apiGroups("management.cattle.io").resources("clusters").verbs("create")
	rb.addRole("Manage All Clusters", "manage-clusters").addRule().apiGroups("management.cattle.io").resources("clusters").verbs("*")
	rb.addRole("Manage Node Drivers", "manage-node-drivers").addRule().apiGroups("management.cattle.io").resources("machinedrivers").verbs("*")
	rb.addRole("Manage Catalogs", "manage-catalogs").addRule().apiGroups("management.cattle.io").resources("catalogs", "templates", "templateversions").verbs("*")
	rb.addRole("Use Catalog Templates", "use-catalogs").addRule().apiGroups("management.cattle.io").resources("templates", "templateversions").verbs("get", "list")
	rb.addRole("Manage Users", "manage-users").addRule().apiGroups("management.cattle.io").resources("users", "globalroles", "globalrolebindings").verbs("*")
	rb.addRole("Manage Roles", "manage-roles").addRule().apiGroups("management.cattle.io").resources("roletemplates").verbs("*")
	rb.addRole("Manage Authentication", "manage-authn").addRule().apiGroups("management.cattle.io").resources("authconfigs").verbs("get", "list", "update")
	rb.addRole("Manage Node Templates", "manage-node-templates").addRule().apiGroups("management.cattle.io").resources("machinetemplates").verbs("*")
	rb.addRole("Use Node Templates", "use-node-templates").addRule().apiGroups("management.cattle.io").resources("machinedrivers").verbs("get", "list").
		addRule().apiGroups("management.cattle.io").resources("machinetemplates").verbs("*")
	rb.addRole("Admin", "admin").addRule().apiGroups("*").resources("*").verbs("*").
		addRule().apiGroups().nonResourceURLs("*").verbs("*")

	rb.addRole("User", "user").addRule().apiGroups("management.cattle.io").resources("principals", "roletemplates").verbs("get", "list").
		addRule().apiGroups("management.cattle.io").resources("users").verbs("get", "list").
		addRule().apiGroups("management.cattle.io").resources("preferences").verbs("*").
		addRule().apiGroups("management.cattle.io").resources("clusters").verbs("create").
		addRule().apiGroups("management.cattle.io").resources("templates", "templateversions").verbs("get", "list").
		addRule().apiGroups("management.cattle.io").resources("machinedrivers").verbs("get", "list").
		addRule().apiGroups("management.cattle.io").resources("machinetemplates").verbs("*")

	rb.addRole("User Base", "user-base").addRule().apiGroups("management.cattle.io").resources("principals", "roletemplates").verbs("get", "list").
		addRule().apiGroups("management.cattle.io").resources("users").verbs("get", "list").
		addRule().apiGroups("management.cattle.io").resources("preferences").verbs("*")

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

	rb.addRoleTemplate("Cluster Owner", "cluster-owner", "cluster", true, false, false).
		addRule().apiGroups("*").resources("*").verbs("*").
		addRule().apiGroups().nonResourceURLs("*").verbs("*")

	rb.addRoleTemplate("Create Projects", "create-projects", "cluster", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projects").verbs("create")

	rb.addRoleTemplate("Project Owner", "project-owner", "project", true, false, false).
		addRule().apiGroups("management.cattle.io").resources("projectroletemplatebindings").verbs("*").
		addRule().apiGroups("project.cattle.io").resources("worklods").verbs("*").
		addRule().apiGroups("").resources("namespaces").verbs("create").
		setRoleTemplateNames("admin", "create-ns")

	rb.addRoleTemplate("Project Member", "project-member", "project", true, false, false).
		addRule().apiGroups("project.cattle.io").resources("worklods").verbs("*").
		setRoleTemplateNames("edit", "create-ns")

	rb.addRoleTemplate("Read-only", "read-only", "project", true, false, false).
		addRule().apiGroups("project.cattle.io").resources("worklods").verbs("get", "list").
		setRoleTemplateNames("view")

	rb.addRoleTemplate("Create Namespaces", "create-ns", "project", true, false, true).
		addRule().apiGroups("").resources("namespaces").verbs("create")

	if err := rb.reconcileRoleTemplates(management); err != nil {
		return errors.Wrap(err, "problem reconciling role templates")
	}

	management.Management.GlobalRoleBindings("").Create(
		&v3.GlobalRoleBinding{
			ObjectMeta: v1.ObjectMeta{
				Name: "admin",
			},
			Subject: rbacv1.Subject{
				Kind: "User",
				Name: "admin",
			},
			GlobalRoleName: "admin",
		})

	return addMachineDrivers(management)
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
