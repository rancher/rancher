package vsphere

import (
	"context"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

func soapLister(ctx context.Context, fieldName string, cc *v1.Secret, dc string) ([]string, error) {
	finder, err := getSoapFinder(ctx, cc, dc)
	if err != nil {
		return nil, err
	}

	var data []string
	switch fieldName {
	case "clusters":
		data, err = listClusters(ctx, finder) //tbd
	case "virtual-machines":
		data, err = listVirtualMachines(ctx, finder, "virtual-machines")
	case "templates":
		data, err = listVirtualMachines(ctx, finder, "templates")
	case "data-stores":
		data, err = listDataStores(ctx, finder)
	case "folders":
		data, err = listFolders(ctx, finder)
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

func tagsLister(ctx context.Context, fieldName string, cc *v1.Secret, cat string) ([]string, error) {
	tagsManager, err := getTagsManager(ctx, cc)
	if err != nil {
		return nil, err
	}

	var data []string
	switch fieldName {
	case "tags":
		data, err = listTags(ctx, tagsManager, cat)
	case "tag-categories":
		data, err = listTagCategories(ctx, tagsManager)
	}

	return data, err
}

func libraryLister(ctx context.Context, fieldName string, cc *v1.Secret, library string) ([]string, error) {
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

	var data []string
	for _, lib := range libs {
		data = append(data, fmt.Sprintf("/%s/%s", lib.ID, lib.Name))
	}

	return data, nil
}

func listContentLibraryTemplates(ctx context.Context, mgr *library.Manager, library string) ([]string, error) {
	items, err := mgr.GetLibraryItems(ctx, library)
	if err != nil {
		return nil, err
	}

	var data []string
	for _, item := range items {
		if item.Type == "ovf" {
			data = append(data, item.Name)
		}
	}

	return data, nil
}

func listCustomFields(ctx context.Context, cc *v1.Secret) ([]string, error) {
	mgr, err := getCustomFieldsManager(ctx, cc)
	if err != nil {
		return nil, err
	}

	field, err := mgr.Field(ctx)
	if err != nil {
		return nil, err
	}

	var data []string
	for _, def := range field {
		data = append(data, def.Name)
	}
	return data, nil
}

func listTags(ctx context.Context, tagsManager *tags.Manager, cat string) ([]string, error) {
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

	var data []string
	for _, tag := range lsTags {
		data = append(data, fmt.Sprintf("%s/%s", m[tag.CategoryID].Name, tag.Name))
	}

	return data, nil
}

func listTagCategories(ctx context.Context, tagsManager *tags.Manager) ([]string, error) {
	categories, err := tagsManager.GetCategories(ctx)
	if err != nil {
		return nil, err
	}

	var data []string
	for _, cat := range categories {
		data = append(data, cat.Name)
	}

	return data, nil
}

func listClusters(ctx context.Context, finder *find.Finder) ([]string, error) {
	clusters, err := finder.ClusterComputeResourceList(ctx, "*")
	if err != nil {
		return nil, err
	}

	var data []string
	fmt.Println("asdf")
	fmt.Printf("%#v\n", clusters)
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

func listFolders(ctx context.Context, finder *find.Finder) ([]string, error) {
	folders, err := finder.FolderList(ctx, "*")
	if err != nil {
		return nil, err
	}

	var data []string
	for _, f := range folders {
		data = append(data, f.InventoryPath)
	}

	return data, nil
}

func listHosts(ctx context.Context, finder *find.Finder) ([]string, error) {
	hosts, err := finder.HostSystemList(ctx, "*")
	if err != nil {
		return nil, err
	}

	var data []string
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
		data = append(data, getNetworkInventoryPath(net))
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

	return data, nil
}

func getNetworkInventoryPath(nr object.NetworkReference) string {
	var r string
	switch nr.Reference().Type {
	case "Network":
		r = nr.(*object.Network).InventoryPath
	case "OpaqueNetwork":
		r = nr.(*object.OpaqueNetwork).InventoryPath
	case "DistributedVirtualPortgroup":
		r = nr.(*object.DistributedVirtualPortgroup).InventoryPath
	case "DistributedVirtualSwitch":
		r = nr.(*object.DistributedVirtualSwitch).InventoryPath
	case "VmwareDistributedVirtualSwitch":
		r = nr.(*object.VmwareDistributedVirtualSwitch).InventoryPath
	}

	return r
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
	c, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func getContentLibraryManager(ctx context.Context, cc *corev1.Secret) (*library.Manager, error) {
	soap, err := getSoapClient(ctx, cc)
	if err != nil {
		return nil, err
	}

	mgr := library.NewManager(rest.NewClient(soap.Client))
	ui := url.UserPassword(string(cc.Data[dataFields["username"]]), string(cc.Data[dataFields["password"]]))
	err = mgr.Login(ctx, ui)
	if err != nil {
		return nil, err
	}

	return mgr, nil
}

func getTagsManager(ctx context.Context, cc *corev1.Secret) (*tags.Manager, error) {
	soap, err := getSoapClient(ctx, cc)
	if err != nil {
		return nil, err
	}

	mgr := tags.NewManager(rest.NewClient(soap.Client))
	ui := url.UserPassword(string(cc.Data[dataFields["username"]]), string(cc.Data[dataFields["password"]]))
	err = mgr.Login(ctx, ui)
	if err != nil {
		return nil, err
	}

	return mgr, nil
}

func getCustomFieldsManager(ctx context.Context, cc *corev1.Secret) (*object.CustomFieldsManager, error) {
	soap, err := getSoapClient(ctx, cc)
	if err != nil {
		return nil, err
	}

	mgr, err := object.GetCustomFieldsManager(soap.Client)
	if err != nil {
		return nil, err
	}

	return mgr, nil
}
