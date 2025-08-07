package management

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"

	"github.com/docker/docker/pkg/reexec"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/auth/providers/local/pbkdf2"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/urfave/cli"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
)

func RegisterPasswordResetCommand() {
	reexec.Register("/usr/bin/reset-password", resetPassword)
	reexec.Register("reset-password", resetPassword)
}

const (
	length     = 20
	cost       = 10
	characters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
)

func resetPassword() {
	app := cli.NewApp()
	app.Description = "Reset the password for the default admin user"

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

		set := labels.Set(map[string]string{"authz.management.cattle.io/bootstrapping": "admin-user"})
		admins, err := client.Users("").List(v1.ListOptions{LabelSelector: set.String()})
		if err != nil {
			return errors.Errorf("Couldn't get default admin user. %v", err)
		}

		count := len(admins.Items)
		if count != 1 {
			var users []string
			for _, u := range admins.Items {
				users = append(users, u.Name)
			}
			return errors.Errorf("%v users were found with %v label. They are %v. Can only reset the default admin password when there is exactly one user with this label",
				count, set, users)
		}

		admin := admins.Items[0]
		pass := generatePassword(length)
		wranglerContext, err := wrangler.NewContext(context.Background(), nil, conf)
		if err != nil {
			return fmt.Errorf("couldn't create wrangler context %w", err)
		}
		err = wranglerContext.ControllerFactory.SharedCacheFactory().StartGVK(context.Background(), schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Secret",
		})
		wranglerContext.ControllerFactory.SharedCacheFactory().WaitForCacheSync(context.Background())
		if err != nil {
			return fmt.Errorf("couldn't start wrangler cache for secrets %w", err)
		}
		pwdCreator := pbkdf2.New(wranglerContext.Core.Secret().Cache(), wranglerContext.Core.Secret())
		if err := pwdCreator.UpdatePassword(admin.Name, string(pass)); err != nil {
			return fmt.Errorf("couldn't update password %w", err)
		}
		fmt.Fprintf(os.Stdout, "New password for default admin user (%v):\n%s\n", admin.Name, pass)

		admin.MustChangePassword = true
		_, err = wranglerContext.Mgmt.User().Update(&admin)
		if err != nil {
			return fmt.Errorf("couldn't update user must change password %w", err)
		}

		return err
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generatePassword(length int) []byte {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		log.Fatal(err)
	}

	out := make([]byte, length)
	for i := range out {
		index := uint8(bytes[i]) % uint8(len(characters))
		out[i] = characters[index]
	}

	return out
}
