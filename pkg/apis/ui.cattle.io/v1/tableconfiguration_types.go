package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TableConfigurationType says what a TableConfiguration holds.
// +kubebuilder:validation:Enum=Page;View
type TableConfigurationType string

const (
	// TableConfigurationTypePage holds page level configuration. The UI loads only one of these per page.
	TableConfigurationTypePage TableConfigurationType = "Page"
	// TableConfigurationTypeView holds a single table configuration, and the UI supplements the page with these.
	TableConfigurationTypeView TableConfigurationType = "View"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,shortName=tablecfg
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="Page",type="string",JSONPath=".spec.page"
// +kubebuilder:printcolumn:name="View",type="string",JSONPath=".spec.view.name"
// +kubebuilder:printcolumn:name="Default",type="string",JSONPath=".spec.defaultViewId"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// TableConfiguration is a configuration of the UI's resource tables shared with every user. Each user's own
// configuration is persisted elsewhere; this is the one everyone starts from.
type TableConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata; More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the table configuration.
	Spec TableConfigurationSpec `json:"spec"`
}

// TableConfigurationSpec is what is persisted. Either page level configuration (Page) or a single
// table configuration (View).
// +kubebuilder:validation:XValidation:rule="self.type == 'Page' ? has(self.views) : !has(self.views)",message="views is required when type is Page and not allowed when type is View"
// +kubebuilder:validation:XValidation:rule="self.type == 'View' ? has(self.view) : !has(self.view)",message="view is required when type is View and not allowed when type is Page"
// +kubebuilder:validation:XValidation:rule="self.type == 'Page' || (!has(self.defaultViewId) && !has(self.allIndex))",message="defaultViewId and allIndex are only allowed when type is Page"
type TableConfigurationSpec struct {
	// Type is Page for page level configuration, and the UI will load only one of these per page. View
	// contains a single table configuration, and the UI will supplement the page with these.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="type is immutable"
	Type TableConfigurationType `json:"type"`

	// Page is the location to apply customization. Could relate to a resource type (e.g.
	// provisioning.cattle.io.cluster) or a specific name like 'home'.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="page is immutable"
	Page string `json:"page"`

	// View is a saved table configuration (SavedView). Only for View.
	// +optional
	View *TableViewSaved `json:"view,omitempty"`

	// Views are the page's saved table configurations, in tab order (SavedPage). Required for Page.
	// +optional
	// +listType=map
	// +listMapKey=id
	Views []TableViewSaved `json:"views,omitempty"`

	// DefaultViewID is the id of the view selected when the page opens. Only for Page.
	// +optional
	// +nullable
	DefaultViewID *string `json:"defaultViewId,omitempty"`

	// AllIndex is where the table's own tab sits among the saved ones. Missing means the front. Only for Page.
	// +optional
	// +kubebuilder:validation:Minimum=0
	AllIndex *int32 `json:"allIndex,omitempty"`
}

// TableViewSaved is a saved table configuration.
type TableViewSaved struct {
	// ID identifies the view, among a page's views and in each user's views.
	ID string `json:"id"`

	// Name is the view's name, shown on its tab.
	Name string `json:"name"`

	// Query filters the table, as typed into its filter.
	// +optional
	Query string `json:"query,omitempty"`

	// Columns are the column ids shown. Missing or null means the table's default columns.
	// +optional
	// +nullable
	Columns []string `json:"columns,omitempty"`

	// ColumnOrder are the column ids in order. Missing or null means the table's own order.
	// +optional
	// +nullable
	ColumnOrder []string `json:"columnOrder,omitempty"`

	// LabelColumns are the label keys shown as columns.
	// +optional
	LabelColumns []string `json:"labelColumns,omitempty"`

	// GroupBy is the field id to group by, or 'none' to turn off the table's default grouping.
	// +optional
	// +nullable
	GroupBy *string `json:"groupBy,omitempty"`

	// Sort is the column the table is sorted by. Missing or null means the table's own sort.
	// +optional
	// +nullable
	Sort *string `json:"sort,omitempty"`

	// SortDescending sorts the table in descending order.
	// +optional
	SortDescending bool `json:"sortDescending,omitempty"`
}
