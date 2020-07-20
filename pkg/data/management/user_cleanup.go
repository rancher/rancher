package management

import (
	"context"
	"fmt"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
)

type userCleanup struct {
	users         v3.UserInterface
	userLister    v3.UserLister
	clusterLister v3.ClusterLister
	projectLister v3.ProjectLister
}

func CleanupOrphanedSystemUsers(ctx context.Context, management *config.ManagementContext) {
	u := userCleanup{
		users:         management.Management.Users(""),
		userLister:    management.Management.Users("").Controller().Lister(),
		clusterLister: management.Management.Clusters("").Controller().Lister(),
		projectLister: management.Management.Projects("").Controller().Lister(),
	}

	cleanupCtx, cleanupCancel := context.WithCancel(ctx)
	go func(context.Context, context.CancelFunc) {
		wait.PollImmediate(time.Hour*24, 0, func() (bool, error) {
			logrus.Debugf("Starting orphaned system users cleanup with exponentialBackoff")
			steps := 5
			backOffDuration := time.Minute * 10
			factor := 2
			var err error
			for steps > 0 {
				err = u.cleanup()
				if err != nil {
					time.Sleep(backOffDuration)
					backOffDuration = time.Duration(factor) * backOffDuration
				} else {
					break
				}
				steps--
			}
			if err != nil {
				// returning false & nil because PollImmediate terminates on error
				logrus.Error(err)
				return false, nil
			}
			// no error returned, user cleanup done, calling the child context's cancelfunc to terminate child context
			cleanupCancel()
			return true, nil
		})
	}(cleanupCtx, cleanupCancel)
}

func (u *userCleanup) cleanup() error {
	users, err := u.userLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("error listing users during system account users cleanup: %v", err)
	}
	clusters, err := u.clusterLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("error listing clusters during system account users cleanup: %v", err)
	}
	var returnErr error
	for _, user := range users {
		systemUser := false
		for _, principal := range user.PrincipalIDs {
			if strings.HasPrefix(principal, "system://") {
				systemUser = true
				break
			}
		}
		if !systemUser {
			continue
		}
		if err := u.checkClusterOrProjectExistsForSystemUser(user, clusters); err != nil {
			// errors are logged in checkClusterOrProjectExistsForSystemUser, but continue for loop for other users
			// returning only one error at the end is sufficient for ExponentialBackoff/PollImmediate to try this again
			returnErr = err
		}
	}
	return returnErr
}

func (u *userCleanup) checkClusterOrProjectExistsForSystemUser(user *v3.User, clusters []*v3.Cluster) error {
	displayName := user.DisplayName
	if strings.HasPrefix(user.DisplayName, systemaccount.ClusterSystemAccountPrefix) {
		clusterID := strings.TrimPrefix(displayName, systemaccount.ClusterSystemAccountPrefix)
		// check if this cluster exists, if not, delete this user
		clusterExists := false
		for _, cluster := range clusters {
			if cluster.Name == clusterID {
				clusterExists = true
				break
			}
		}
		if clusterExists {
			return nil
		}
		// cluster not found, delete the system user
		return u.deleteSystemUser(user.Name)
	} else if strings.HasPrefix(user.DisplayName, systemaccount.ProjectSystemAccountPrefix) {
		projectID := strings.TrimPrefix(displayName, systemaccount.ProjectSystemAccountPrefix)
		// check if this project exists, if not, delete this user
		// how to find the cluster ID of this project: go through all clusters
		projectExists := false
		for _, cluster := range clusters {
			project, err := u.projectLister.Get(cluster.Name, projectID)
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("error finding project %v during system account users cleanup: %v", projectID, err)
			} else if err == nil && project != nil {
				projectExists = true
				break
			}
		}
		if projectExists {
			return nil
		}
		// project not found, so delete the system user for this project
		return u.deleteSystemUser(user.Name)
	}
	return nil
}

func (u *userCleanup) deleteSystemUser(userName string) error {
	err := u.users.Delete(userName, &v1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) && !errors.IsGone(err) {
		return err
	}
	logrus.Debugf("Deleted system user %v since its associated cluster/project no longer exists", userName)
	return nil
}
