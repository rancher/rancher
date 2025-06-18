package management

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/docker/pkg/reexec"
	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/rancher/pkg/auth/providers/local/pbkdf2"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/urfave/cli"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

func RegisterEnsureDefaultAdminCommand() {
	reexec.Register("/usr/bin/ensure-default-admin", ensureDefaultAdmin)
	reexec.Register("ensure-default-admin", ensureDefaultAdmin)
}

func ensureDefaultAdmin() {
	app := cli.NewApp()
	app.Description = "Ensure an available default admin user"

	app.Action = func(c *cli.Context) error {
		kubeConfigPath := os.ExpandEnv("$HOME/.kube/config")
		if _, err := os.Stat(kubeConfigPath); err != nil {
			kubeConfigPath = ""
		}

		conf, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			return fmt.Errorf("Couldn't get kubeconfig. %v", err)
		}

		client, err := v3.NewForConfig(*conf)
		if err != nil {
			return errors.Errorf("Couldn't get kubernetes client. %v", err)
		}

		users, err := client.Users("").List(v1.ListOptions{})
		if err != nil {
			return errors.Errorf("Error fetching users. %v", err)
		}

		var admins []v3.User
		for _, u := range users.Items {
			if u.Username == "admin" {
				admins = append(admins, u)
			}
		}

		count := len(admins)
		if count > 1 {
			var adminNames []string
			for _, u := range admins {
				adminNames = append(adminNames, u.Name)
			}
			return errors.Errorf("%v users were found with the name \"admin\". They are %v. Can only reset the default admin password when there is exactly one user with this label",
				count, adminNames)
		} else if count == 1 {
			admin := admins[0]
			fmt.Fprintf(os.Stdout, "Found existing default admin user (%v)\n", admin.Name)

			enabledChanged := ensureAdminIsEnabled(&admin)
			labelingChanged := ensureAdminIsLabeled(&admin)

			if enabledChanged || labelingChanged {
				_, err = client.Users("").Update(&admin)
			}
			if err != nil {
				return errors.Errorf("Error updating user. %v", err)
			}
			err = ensureAdminIsAdmin(client, admin)
			if err != nil {
				return errors.Errorf("Couldn't make existing \"admin\" an actual admin. %v", err)
			}

		} else {
			wranglerContext, err := wrangler.NewContext(context.TODO(), nil, conf)
			if err != nil {
				return err
			}

			err = createNewAdmin(client, length, wranglerContext.Core.Secret().Cache(), wranglerContext.Core.Secret(), wranglerContext.Mgmt.User().Cache())
			if err != nil {
				return errors.Errorf("Couldn't create a new admin. %v", err)
			}
		}

		return err
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func createNewAdmin(client v3.Interface, length int, secretLister wranglerv1.SecretCache, secretClient wranglerv1.SecretClient, userLister mgmtcontrollers.UserCache) error {
	pass := generatePassword(length)

	admin, err := client.Users("").Create(&v3.User{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "user-",
			Labels:       defaultAdminLabel,
		},
		DisplayName:        "Default Admin",
		Username:           "admin",
		MustChangePassword: false,
	})

	if err != nil {
		return err
	}

	pwdCreator := pbkdf2.New(secretLister, secretClient)
	if err := pwdCreator.CreatePassword(admin, string(pass)); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, err.Error())
	}

	addAdminRoleToUser(client, *admin)

	fmt.Fprintf(os.Stdout, "New default admin user (%v):\n", admin.Name)
	fmt.Fprintf(os.Stdout, "New password for default admin user (%v):\n%s\n", admin.Name, pass)
	return err
}

func ensureAdminIsEnabled(admin *v3.User) bool {
	if admin.Enabled == nil || *admin.Enabled {
		fmt.Fprintf(os.Stdout, "Existing default admin user (%v) is already enabled\n", admin.Name)
		return false
	}

	_true := true
	admin.Enabled = &_true
	fmt.Fprintf(os.Stdout, "Enabling existing default admin user (%v)\n", admin.Name)
	return true
}

func ensureAdminIsAdmin(client v3.Interface, admin v3.User) error {
	bindings, err := client.GlobalRoleBindings("").List(v1.ListOptions{})
	if err != nil {
		return err
	}

	for _, b := range bindings.Items {
		if b.UserName == admin.Name && b.GlobalRoleName == "admin" {
			fmt.Fprintf(os.Stdout, "Existing default admin user (%v) is already an admin\n", admin.Name)
			return nil
		}
	}

	fmt.Fprintf(os.Stdout, "Giving existing default admin user (%v) admin permissions\n", admin.Name)
	return addAdminRoleToUser(client, admin)
}

func ensureAdminIsLabeled(admin *v3.User) bool {
	changed := true
	if current, exists := admin.ObjectMeta.Labels[defaultAdminLabelKey]; exists {
		changed = current != defaultAdminLabelValue
	}

	if changed {
		fmt.Fprintf(os.Stdout, "Labeling existing default admin user (%v) as admin\n", admin.Name)

		admin.ObjectMeta.Labels[defaultAdminLabelKey] = defaultAdminLabelValue
	} else {
		fmt.Fprintf(os.Stdout, "Existing default admin user (%v) already labeled as admin\n", admin.Name)
	}

	return changed
}

func addAdminRoleToUser(client v3.Interface, admin v3.User) error {
	_, err := client.GlobalRoleBindings("").Create(
		&v3.GlobalRoleBinding{
			ObjectMeta: v1.ObjectMeta{
				GenerateName: "globalrolebinding-",
				Labels:       defaultAdminLabel,
			},
			UserName:       admin.Name,
			GlobalRoleName: "admin",
		})

	return err
}
