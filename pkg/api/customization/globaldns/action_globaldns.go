package globaldns

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	gaccess "github.com/rancher/rancher/pkg/api/customization/globalnamespaceaccess"
	managementschema "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3/schema"
	client "github.com/rancher/rancher/pkg/types/client/management/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	addProjectsAction    = "addProjects"
	removeProjectsAction = "removeProjects"
)

var backoff = wait.Backoff{
	Duration: 100 * time.Millisecond,
	Factor:   2,
	Jitter:   0.5,
	Steps:    7,
}

func (w *Wrapper) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, addProjectsAction)
	resource.AddAction(apiContext, removeProjectsAction)
}

func (w *Wrapper) ActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if err := access.ByID(request, &managementschema.Version, client.GlobalDnsType, request.ID, &client.GlobalDns{}); err != nil {
		return err
	}
	split := strings.SplitN(request.ID, ":", 2)
	if len(split) != 2 {
		return fmt.Errorf("incorrect global DNS ID")
	}
	gDNS, err := w.GlobalDNSes.GetNamespaced(split[0], split[1], v1.GetOptions{})
	if err != nil {
		return err
	}
	callerID := request.Request.Header.Get(gaccess.ImpersonateUserHeader)
	metaAccessor, err := meta.Accessor(gDNS)
	if err != nil {
		return err
	}
	creatorID, ok := metaAccessor.GetAnnotations()[creatorIDAnn]
	if !ok {
		return fmt.Errorf("GlobalDNS %v has no creatorId annotation", metaAccessor.GetName())
	}
	ma := gaccess.MemberAccess{
		Users:     w.Users,
		GrbLister: w.GrbLister,
		GrLister:  w.GrLister,
	}
	accessType, err := ma.GetAccessTypeOfCaller(callerID, creatorID, gDNS.Name, gDNS.Spec.Members)
	if err != nil {
		return err
	}
	if accessType != gaccess.OwnerAccess {
		return fmt.Errorf("only owners can modify global DNS target projects")
	}

	actionInput, err := parse.ReadBody(request.Request)
	if err != nil {
		return err
	}
	inputProjects := convert.ToStringSlice(actionInput[client.UpdateGlobalDNSTargetsInputFieldProjectIDs])
	switch actionName {
	case addProjectsAction:
		return w.addProjects(request, inputProjects)
	case removeProjectsAction:
		return w.removeProjects(request, inputProjects)
	default:
		return fmt.Errorf("bad action for global dns %v", actionName)
	}
}

func (w *Wrapper) addProjects(request *types.APIContext, inputProjects []string) error {
	ma := gaccess.MemberAccess{
		Users:     w.Users,
		GrbLister: w.GrbLister,
		GrLister:  w.GrLister,
	}
	if err := ma.CheckCallerAccessToTargets(request, inputProjects, client.ProjectType, &client.Project{}); err != nil {
		return err
	}

	split := strings.SplitN(request.ID, ":", 2)
	if len(split) != 2 {
		return fmt.Errorf("incorrect global DNS ID")
	}

	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		existingProjects := make(map[string]bool)
		gDNS, err := w.GlobalDNSes.GetNamespaced(split[0], split[1], v1.GetOptions{})
		if err != nil {
			return false, err
		}
		if gDNS.Spec.MultiClusterAppName != "" {
			return false, httperror.NewAPIError(httperror.InvalidOption,
				fmt.Sprintf("cannot add projects to globaldns as targets if multiclusterappID is set %s", gDNS.Spec.MultiClusterAppName))
		}
		for _, p := range gDNS.Spec.ProjectNames {
			existingProjects[p] = true
		}
		for _, p := range inputProjects {
			if existingProjects[p] {
				return false, httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("duplicate projects in targets %s", p))
			}
			existingProjects[p] = true
		}
		for _, name := range inputProjects {
			gDNS.Spec.ProjectNames = append(gDNS.Spec.ProjectNames, name)
		}
		_, err = w.GlobalDNSes.Update(gDNS)
		if err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})

	if err != nil {
		return err
	}
	op := map[string]interface{}{
		"message": "addedProjects",
	}
	request.WriteResponse(http.StatusOK, op)
	return nil
}

func (w *Wrapper) removeProjects(request *types.APIContext, inputProjects []string) error {
	split := strings.SplitN(request.ID, ":", 2)
	if len(split) != 2 {
		return fmt.Errorf("incorrect global DNS ID")
	}

	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		gDNS, err := w.GlobalDNSes.GetNamespaced(split[0], split[1], v1.GetOptions{})
		if err != nil {
			return false, err
		}
		toRemoveProjects := make(map[string]bool)
		var finalProjects []string
		for _, p := range inputProjects {
			toRemoveProjects[p] = true
		}
		for _, p := range gDNS.Spec.ProjectNames {
			if !toRemoveProjects[p] {
				finalProjects = append(finalProjects, p)
			}
		}
		gDNS.Spec.ProjectNames = finalProjects
		_, err = w.GlobalDNSes.Update(gDNS)
		if err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return err
	}
	op := map[string]interface{}{
		"message": "removedProjects",
	}
	request.WriteResponse(http.StatusOK, op)
	return nil
}
