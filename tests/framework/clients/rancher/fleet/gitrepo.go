package fleet

import (
	"context"

	v1alpha1 "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Create is GitRepoInterface's Create function, that is being overwritten to register its delete function to the session.Session
// that is being reference.
func (g *GitRepo) Create(ctx context.Context, gitRepo *v1alpha1.GitRepo, opts metav1.CreateOptions) (*v1alpha1.GitRepo, error) {
	g.ts.RegisterCleanupFunc(func() error {
		err := g.Delete(context.TODO(), gitRepo.GetName(), metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			return nil
		}

		return err
	})
	return g.GitRepoInterface.Create(ctx, gitRepo, opts)
}
