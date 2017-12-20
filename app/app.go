package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/pkg/errors"
	managementController "github.com/rancher/cluster-controller/controller"
	"github.com/rancher/norman/signal"
	"github.com/rancher/rancher/server"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"golang.org/x/crypto/bcrypt"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func Run(kubeConfig rest.Config) error {
	management, err := config.NewManagementContext(kubeConfig)
	if err != nil {
		return err
	}
	management.LocalConfig = &kubeConfig

	handler, err := server.New(context.Background(), management)
	if err != nil {
		return err
	}

	ctx := signal.SigTermCancelContext(context.Background())
	go func() {
		<-ctx.Done()
		if ctx.Err() != nil {
			log.Fatal(ctx.Err())
		}
		os.Exit(1)
	}()

	managementController.Register(ctx, management)

	management.Start(ctx)

	if err := addData(management); err != nil {
		return err
	}

	fmt.Println("Listening on 0.0.0.0:1234")
	return http.ListenAndServe("0.0.0.0:1234", handler)
}

func addData(management *config.ManagementContext) error {
	management.Management.Clusters("").Create(&v3.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: "local",
		},
		Spec: v3.ClusterSpec{
			Internal: true,
		},
	})

	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	admin, err := management.Management.Users("").Create(&v3.User{
		ObjectMeta: v1.ObjectMeta{
			Name: "admin",
		},
		DisplayName:        "Default Admin",
		UserName:           "admin",
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
	rb.addRole("admin").addRule().apiGroups("*").verbs("*").resources("*").
		addRule().apiGroups().nonResourceURLs("*").verbs("*")
	rb.addRole("user").
		addRule().apiGroups("management.cattle.io").resources("clusters").verbs("create")
	if err := rb.reconcileGlobalRoles(management); err != nil {
		return errors.Wrap(err, "problem reconciling globl roles")
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

	return nil
}
