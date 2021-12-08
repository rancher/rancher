package vsphere

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

func processSoapFinder(ctx context.Context, fieldName string, cc *v1.Secret, dc string) ([]string, error) {
	finder, err := getSoapFinder(ctx, cc, dc)
	if err != nil {
		return nil, err
	}

	var data []string
	switch fieldName {
	case "clusters":
		data, err = listClusters(ctx, finder)
	case "virtual-machines":
		data, err = listVirtualMachines(ctx, finder, "virtual-machines")
	case "templates":
		data, err = listVirtualMachines(ctx, finder, "templates")
	case "data-stores":
		data, err = listDataStores(ctx, finder)
	case "data-store-clusters":
		data, err = listDataStoreClusters(ctx, finder)
	case "folders":
		data, err = listFolders(ctx, finder, dc)
	case "hosts":
		data, err = listHosts(ctx, finder)
	case "networks":
		data, err = listNetworks(ctx, finder)
	case "data-centers":
		data, err = listDataCenters(ctx, finder)
	case "resource-pools":
		data, err = listResourcePools(ctx, finder)
	}

	return data, err
}

func processTagsManager(ctx context.Context, fieldName string, cc *v1.Secret, cat string) ([]map[string]string, error) {
	tagsManager, err := getTagsManager(ctx, cc)
	if err != nil {
		return nil, err
	}

	var data []map[string]string
	switch fieldName {
	case "tags":
		data, err = listTags(ctx, tagsManager, cat)
	case "tag-categories":
		data, err = listTagCategories(ctx, tagsManager)
	}

	return data, err
}

func processContentLibraryManager(ctx context.Context, fieldName string, cc *v1.Secret, library string) ([]string, error) {
	libraryManager, err := getContentLibraryManager(ctx, cc)
	if err != nil {
		return nil, err
	}

	var data []string
	switch fieldName {
	case "library-templates":
		data, err = listContentLibraryTemplates(ctx, libraryManager, library)
	case "content-libraries":
		data, err = listContentLibraries(ctx, libraryManager)
	}

	return data, err
}

func listContentLibraries(ctx context.Context, mgr *library.Manager) ([]string, error) {
	libs, err := mgr.GetLibraries(ctx)
	if err != nil {
		return nil, err
	}

	data := []string{""}
	for _, lib := range libs {
		data = append(data, lib.Name)
	}

	return data, nil
}

func listContentLibraryTemplates(ctx context.Context, mgr *library.Manager, library string) ([]string, error) {
	lib, err := mgr.GetLibraryByName(ctx, library)
	if err != nil {
		return nil, err
	}

	items, err := mgr.GetLibraryItems(ctx, lib.ID)
	if err != nil {
		return nil, err
	}

	var data []string
	for _, item := range items {
		if item.Type == "ovf" {
			data = append(data, item.Name)
		}
	}

	sort.Strings(data)
	return data, nil
}

func processCustomFieldsFinder(ctx context.Context, cc *v1.Secret) ([]map[string]interface{}, error) {
	mgr, err := getCustomFieldsManager(ctx, cc)
	if err != nil {
		return nil, err
	}

	field, err := mgr.Field(ctx)
	if err != nil {
		return nil, err
	}

	var data []map[string]interface{}
	for _, def := range field {
		if def.ManagedObjectType != "VirtualMachine" {
			continue
		}
		f := map[string]interface{}{
			"key":  def.Key,
			"name": def.Name,
		}
		data = append(data, f)
	}
	return data, nil
}

func listTags(ctx context.Context, tagsManager *tags.Manager, cat string) ([]map[string]string, error) {
	var lsTags []tags.Tag
	var err error
	if cat == "" {
		lsTags, err = tagsManager.GetTags(ctx)
	} else {
		lsTags, err = tagsManager.GetTagsForCategory(ctx, cat)
	}

	if err != nil {
		return nil, err
	}

	categories, err := tagsManager.GetCategories(ctx)
	if err != nil {
		return nil, err
	}

	m := make(map[string]tags.Category)
	for _, category := range categories {
		m[category.ID] = category
	}

	var data []map[string]string
	for _, tag := range lsTags {
		t := map[string]string{
			"id":       tag.ID,
			"name":     tag.Name,
			"category": m[tag.CategoryID].Name,
		}
		data = append(data, t)
	}

	return data, nil
}

func listTagCategories(ctx context.Context, tagsManager *tags.Manager) ([]map[string]string, error) {
	categories, err := tagsManager.GetCategories(ctx)
	if err != nil {
		return nil, err
	}

	var data []map[string]string
	for _, cat := range categories {
		tc := map[string]string{
			"id":                  cat.ID,
			"name":                cat.Name,
			"multipleCardinality": cat.Cardinality,
		}
		data = append(data, tc)
	}

	return data, nil
}

func listClusters(ctx context.Context, finder *find.Finder) ([]string, error) {
	clusters, err := finder.ClusterComputeResourceList(ctx, "*")
	if err != nil {
		return nil, err
	}

	var data []string
	for _, cluster := range clusters {
		data = append(data, cluster.InventoryPath)
	}

	return data, nil
}

func listDataCenters(ctx context.Context, finder *find.Finder) ([]string, error) {
	dcs, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, err
	}

	var data []string
	for _, v := range dcs {
		data = append(data, v.InventoryPath)
	}

	return data, nil
}

func listResourcePools(ctx context.Context, finder *find.Finder) ([]string, error) {
	dcs, err := finder.ResourcePoolList(ctx, "*")
	if err != nil {
		return nil, err
	}

	var data []string
	for _, v := range dcs {
		data = append(data, v.InventoryPath)
	}

	return data, nil
}

func listDataStores(ctx context.Context, finder *find.Finder) ([]string, error) {
	dataStores, err := finder.DatastoreList(ctx, "*")
	if err != nil {
		return nil, err
	}

	var data []string
	for _, ds := range dataStores {
		data = append(data, ds.InventoryPath)
	}

	return data, nil
}

func listDataStoreClusters(ctx context.Context, finder *find.Finder) ([]string, error) {
	dataStores, err := finder.DatastoreClusterList(ctx, "*")
	if err != nil {
		return nil, err
	}

	var data []string
	for _, ds := range dataStores {
		data = append(data, ds.InventoryPath)
	}

	return data, nil
}

func listFolders(ctx context.Context, finder *find.Finder, dc string) ([]string, error) {
	folders, err := finder.FolderList(ctx, "*")
	if err != nil {
		return nil, err
	}

	prefix := path.Join(dc, "vm")
	// base case of /<datacenter>/vm is covered by ""
	data := []string{""}
	for _, f := range folders {
		if strings.HasPrefix(f.InventoryPath, prefix) {
			data = append(data, f.InventoryPath)
		}
	}

	return data, nil
}

func listHosts(ctx context.Context, finder *find.Finder) ([]string, error) {
	hosts, err := finder.HostSystemList(ctx, "*")
	if err != nil {
		return nil, err
	}

	data := []string{""} //blank default

	for _, h := range hosts {
		data = append(data, h.InventoryPath)
	}

	return data, nil
}

func listNetworks(ctx context.Context, finder *find.Finder) ([]string, error) {
	networks, err := finder.NetworkList(ctx, "*")
	if err != nil {
		return nil, err
	}

	var data []string
	for _, net := range networks {
		data = append(data, net.GetInventoryPath())
	}

	return data, nil
}

func listVirtualMachines(ctx context.Context, finder *find.Finder, t string) ([]string, error) {
	if t == "" {
		t = "both"
	}
	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, err
	}

	var data []string
	for _, vm := range vms {
		switch t {
		case "both":
			data = append(data, vm.InventoryPath)
		case "templates":
			if isTemplate, err := vm.IsTemplate(ctx); err == nil && isTemplate {
				data = append(data, vm.InventoryPath)
			}
		case "virtual-machines":
			if isTemplate, err := vm.IsTemplate(ctx); err == nil && !isTemplate {
				data = append(data, vm.InventoryPath)
			}
		}
	}

	sort.Strings(data)
	return data, nil
}

func getSoapFinder(ctx context.Context, cc *corev1.Secret, dc string) (*find.Finder, error) {
	c, err := getSoapClient(ctx, cc)
	if err != nil {
		return nil, err
	}

	finder := find.NewFinder(c.Client, true)

	if dc != "" {
		dataCenter, err := finder.Datacenter(ctx, dc)
		if err != nil {
			return nil, err
		}

		finder.SetDatacenter(dataCenter)
	}

	return finder, nil
}

func getSoapClient(ctx context.Context, cc *corev1.Secret) (*govmomi.Client, error) {
	u, err := url.Parse(fmt.Sprintf("https://%s:%s/sdk", cc.Data[dataFields["host"]], cc.Data[dataFields["port"]]))
	if err != nil {
		return nil, err
	}

	u.User = url.UserPassword(string(cc.Data[dataFields["username"]]), string(cc.Data[dataFields["password"]]))
	return govmomi.NewClient(ctx, u, true)
}

func getContentLibraryManager(ctx context.Context, cc *corev1.Secret) (*library.Manager, error) {
	soap, err := getSoapClient(ctx, cc)
	if err != nil {
		return nil, err
	}

	mgr := library.NewManager(rest.NewClient(soap.Client))
	ui := url.UserPassword(string(cc.Data[dataFields["username"]]), string(cc.Data[dataFields["password"]]))

	return mgr, mgr.Login(ctx, ui)
}

func getTagsManager(ctx context.Context, cc *corev1.Secret) (*tags.Manager, error) {
	soap, err := getSoapClient(ctx, cc)
	if err != nil {
		return nil, err
	}

	mgr := tags.NewManager(rest.NewClient(soap.Client))
	ui := url.UserPassword(string(cc.Data[dataFields["username"]]), string(cc.Data[dataFields["password"]]))

	return mgr, mgr.Login(ctx, ui)
}

func getCustomFieldsManager(ctx context.Context, cc *corev1.Secret) (*object.CustomFieldsManager, error) {
	soap, err := getSoapClient(ctx, cc)
	if err != nil {
		return nil, err
	}

	return object.GetCustomFieldsManager(soap.Client)
}
