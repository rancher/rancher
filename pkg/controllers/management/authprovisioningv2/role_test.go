package authprovisioningv2

import (
	"testing"

	"github.com/rancher/wrangler/pkg/generic"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	simpleHandler = &handler{
		roleController:               mockRoleController{},
		roleBindingController:        mockRoleBindingController{},
		clusters:                     mockCluster{},
		mgmtClusters:                 mockMgmtCluster{},
		clusterRoleController:        mockClusterRoleController{},
		clusterRoleBindingController: mockClusterRoleBindingController{},
	}

	clusterDeletingAnnotationLabel = "deleting"
	clusterDeletedAnnotationLabel  = "deleted"
	genericAnnotationLabel         = "clusternamespace"
	clusterErrorAnnotationLabel    = "other error"
	noAnnotation                   = map[string]string{}
)

func Test_handler_hasClusterAnnotationsAndDeletionTimestamp(t *testing.T) {
	funcName := "handler.hasClusterAnnotationsAndDeletionTimestamp()"

	tests := []struct {
		name        string
		annotations map[string]string
		wantErr     bool
		want        bool
	}{
		// Enqueued (returned generic.ErrSkip)
		{
			name:        "Cluster not deleted, should requeue",
			annotations: createNameAndNamespaceAnnotation(clusterDeletingAnnotationLabel),
			wantErr:     false,
			want:        true,
		},
		// Other cluster error (clusters.get return err other than IsNotFound)
		{
			name:        "Cluster cache returned unexpected error",
			annotations: createNameAndNamespaceAnnotation(clusterErrorAnnotationLabel),
			wantErr:     true,
			want:        false,
		},
		// cluster is not found (clusters.get returns IsNotFound err)
		{
			name:        "Cluster deleted, can delete",
			annotations: createNameAndNamespaceAnnotation(clusterDeletedAnnotationLabel),
			wantErr:     false,
			want:        false,
		},
		// non-cluster deletion (cluster has no DeletionTimestamp)
		{
			name:        "Other removal reason, can delete",
			annotations: createNameAndNamespaceAnnotation(genericAnnotationLabel),
			wantErr:     false,
			want:        false,
		},
		// no annotations
		{
			name:        "non-annotated, can delete",
			annotations: noAnnotation,
			wantErr:     false,
			want:        false,
		},
	}
	isClusterArg := true
	for i := 0; i < 2; i++ {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// first returned value is always the same role object as the role argument
				enqueue, err := simpleHandler.hasAnnotationsForDeletingCluster(tt.annotations, isClusterArg)
				if enqueue != tt.want {
					t.Errorf("%v returned %v, wanted %v", funcName, enqueue, tt.want)
				}
				if err == nil && tt.wantErr {
					t.Errorf("%v got nil error, wanted %v", funcName, tt.want)
					return
				}
				if err != nil && !tt.wantErr {
					t.Errorf("%v got error = %v, but wanted none", funcName, err)
					return
				}
				if err != nil && tt.wantErr {
					if tt.want && err != generic.ErrSkip {
						t.Errorf("%v wanted ErrSkip, got err = %v", funcName, err)
					}
				}
			})
		}
		isClusterArg = !isClusterArg
	}
}

func Test_OnRemoveRole(t *testing.T) {
	funcName := "OnRemoveRole()"
	tests := []struct {
		name        string
		role        *v1.Role
		wantEnqueue bool
		wantErr     bool
	}{
		{
			name: "Want enqueue",
			role: &v1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: createNameAndNamespaceAnnotation(clusterDeletingAnnotationLabel),
				},
			},
			wantEnqueue: true,
			wantErr:     false,
		},
		{
			name:        "Want no error",
			role:        &v1.Role{},
			wantEnqueue: false,
			wantErr:     false,
		},
		{
			name: "Want error",
			role: &v1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: createNameAndNamespaceAnnotation(clusterErrorAnnotationLabel),
				},
			},
			wantEnqueue: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := simpleHandler.OnRemoveRole("", tt.role)
			if tt.wantEnqueue && err != generic.ErrSkip {
				t.Errorf("%v wanted enqueue, got err = %v", funcName, err)
				return
			}
			if err != nil && !tt.wantEnqueue && !tt.wantErr {
				t.Errorf("%v wanted no error, got err = %v", funcName, err)
			}
		})
	}
}

func Test_OnRemoveRoleBinding(t *testing.T) {
	funcName := "OnRemoveRoleBinding()"
	tests := []struct {
		name        string
		role        *v1.RoleBinding
		wantEnqueue bool
		wantErr     bool
	}{
		{
			name: "Want enqueue",
			role: &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: createNameAndNamespaceAnnotation(clusterDeletingAnnotationLabel),
				},
			},
			wantEnqueue: true,
			wantErr:     false,
		},
		{
			name:        "Want no error",
			role:        &v1.RoleBinding{},
			wantEnqueue: false,
			wantErr:     false,
		},
		{
			name: "Want error",
			role: &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: createNameAndNamespaceAnnotation(clusterErrorAnnotationLabel),
				},
			},
			wantEnqueue: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := simpleHandler.OnRemoveRoleBinding("", tt.role)
			if tt.wantEnqueue && err != generic.ErrSkip {
				t.Errorf("%v wanted enqueue, got err = %v", funcName, err)
				return
			}
			if err != nil && !tt.wantEnqueue && !tt.wantErr {
				t.Errorf("%v wanted no error, got err = %v", funcName, err)
			}
		})
	}
}

func Test_OnRemoveClusterRole(t *testing.T) {
	funcName := "OnRemoveClusterRole()"
	tests := []struct {
		name        string
		role        *v1.ClusterRole
		wantEnqueue bool
		wantErr     bool
	}{
		{
			name: "Want enqueue",
			role: &v1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: createNameAnnotation(clusterDeletingAnnotationLabel),
				},
			},
			wantEnqueue: true,
			wantErr:     false,
		},
		{
			name:        "Want no error",
			role:        &v1.ClusterRole{},
			wantEnqueue: false,
			wantErr:     false,
		},
		{
			name: "Want error",
			role: &v1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: createNameAnnotation(clusterErrorAnnotationLabel),
				},
			},
			wantEnqueue: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := simpleHandler.OnRemoveClusterRole("", tt.role)
			if tt.wantEnqueue && err != generic.ErrSkip {
				t.Errorf("%v wanted enqueue, got err = %v", funcName, err)
				return
			}
			if err != nil && !tt.wantEnqueue && !tt.wantErr {
				t.Errorf("%v wanted no error, got err = %v", funcName, err)
			}
		})
	}
}

func Test_OnRemoveClusterRoleBinding(t *testing.T) {
	funcName := "OnRemoveClusterRoleBinding()"
	tests := []struct {
		name        string
		role        *v1.ClusterRoleBinding
		wantEnqueue bool
		wantErr     bool
	}{
		{
			name: "Want enqueue",
			role: &v1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: createNameAnnotation(clusterDeletingAnnotationLabel),
				},
			},
			wantEnqueue: true,
			wantErr:     false,
		},
		{
			name:        "Want no error",
			role:        &v1.ClusterRoleBinding{},
			wantEnqueue: false,
			wantErr:     false,
		},
		{
			name: "Want error",
			role: &v1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: createNameAnnotation(clusterErrorAnnotationLabel),
				},
			},
			wantEnqueue: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := simpleHandler.OnRemoveClusterRoleBinding("", tt.role)
			if tt.wantEnqueue && err != generic.ErrSkip {
				t.Errorf("%v wanted enqueue, got err = %v", funcName, err)
				return
			}
			if err != nil && !tt.wantEnqueue && !tt.wantErr {
				t.Errorf("%v wanted no error, got err = %v", funcName, err)
			}
		})
	}
}

func createNameAndNamespaceAnnotation(n string) map[string]string {
	return map[string]string{clusterNameLabel: n, clusterNamespaceLabel: n}
}

func createNameAnnotation(n string) map[string]string {
	return map[string]string{clusterNameLabel: n}
}
