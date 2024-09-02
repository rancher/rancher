package crtb

// Condition reason types - Remote controller
const (
	// RemoteBindingsExist is a success indicator. The remote CRTB-related bindings are all present and correct.
	RemoteBindingsExist = "RemoteBindingsExist"
	// RemoteCRTBDeleteOk
	RemoteCRTBDeleteOk = "RemoteCRTBDeleteOk"
	// FailedToDeleteClusterRoleBindings indicates that the controller was unable to delete the CRTB-related cluster role bindings.
	FailedToDeleteClusterRoleBindings = "FailedToDeleteClusterRoleBindings"
	// FailedToDeleteSAImpersonator indicates that the controller was unable to delete the impersonation account for the CRTB's user.
	FailedToDeleteSAImpersonator = "FailedToDeleteSAImpersonator"
	// FailedToEnsureClusterRoleBindings indicates that the controller was unable to create the cluster roles for the role template referenced by the CRTB.
	FailedToEnsureClusterRoleBindings = "FailedToEnsureClusterRoleBindings"
	// FailedToEnsureRoles indicates that the controller was unable to create the roles for the role template referenced by the CRTB.
	FailedToEnsureRoles = "FailedToEnsureRoles"
	// FailedToEnsureSAImpersonator means that the controller was unable to create the impersonation account for the CRTB's user.
	FailedToEnsureSAImpersonator = "FailedToEnsureSAImpersonator"
	// RemoteFailedToGetClusterRoleBindings means that the remote controller was unable to retrieve the CRTB-related cluster role bindings to update.
	RemoteFailedToGetClusterRoleBindings = "RemoteFailedToGetClusterRoleBindings"
	// RemoteFailedToGetLabelRequirements indicates remote issues with the CRTB meta data preventing creation of label requirements.
	RemoteFailedToGetLabelRequirements = "RemoteFailedToGetLabelRequirements"
	// FailedToGetRoleTemplate means that the controller failed to locate the role template referenced by the CRTB.
	FailedToGetRoleTemplate = "FailedToGetRoleTemplate"
	// FailedToGetRoles indicates that the controller failed to locate the roles for the role template referenced by the CRTB.
	FailedToGetRoles = "FailedToGetRoles"
	// RemoteFailedToUpdateCRTBLabels means the remote controller failed to update the CRTB labels indicating success of CRB/RB label updates.
	RemoteFailedToUpdateCRTBLabels = "RemoteFailedToUpdateCRTBLabels"
	// RemoteFailedToUpdateClusterRoleBindings means that the remote controller was unable to properly update the CRTB-related cluster role bindings.
	RemoteFailedToUpdateClusterRoleBindings = "RemoteFailedToUpdateClusterRoleBindings"
	// RemoteLabelsSet is a success indicator. The remote CRTB-related labels are all set.
	RemoteLabelsSet = "RemoteLabelsSet"
)

// Condition reason types - Local controller
const (
	// AuthV2PermissionsOk is a success indicator. The Auth V2 permissions are ok.
	AuthV2PermissionsOk = "AuthV2PermissionsOk"
	// BadRoleReferences indicates issues with the roles referenced by the CRTB.
	BadRoleReferences = "BadRoleReferences"
	// LocalBindingsExist is a success indicator. The local CRTB-related bindings are all present and correct.
	LocalBindingsExist = "LocalBindingsExist"
	// ClusterMembershipBindingForDeleteOk is a success indicator. cluster membership bindings to delete are ok.
	ClusterMembershipBindingForDeleteOk = "ClusterMembershipBindingForDeleteOk"
	// LocalCRTBDeleteOk
	LocalCRTBDeleteOk = "LocalCRTBDeleteOk"
	// FailedClusterMembershipBindingForDelete indicates that CRTB termination failed due to failure to delete the associated cluster membership binding
	FailedClusterMembershipBindingForDelete = "FailedClusterMembershipBindingForDelete"
	// FailedRemovalOfAuthV2Permissions indicates that CRTB termination failed due to failure of removing Auth V2 permissions
	FailedRemovalOfAuthV2Permissions = "FailedRemovalOfAuthV2Permissions"
	// FailedRemovalOfMGMTClusterScopedPrivilegesInProjectNamespace indicates that CRTB termination failed due to failure of removing cluster scoped privileges in the project namespace
	FailedRemovalOfMGMTClusterScopedPrivilegesInProjectNamespace = "FailedRemovalOfMGMTClusterScopedPrivilegesInProjectNamespace"
	// FailedToBuildSubject means that the controller did not have enough data in the CRTB to build a resource reference.
	FailedToBuildSubject = "FailedToBuildSubject"
	// FailedToEnsureClusterMembership means that the controller was unable to create the role binding providing the subject access to the cluster resource.
	FailedToEnsureClusterMembership = "FailedToEnsureClusterMembership"
	// FailedToGetCluster means that the cluster referenced by the CRTB is not found.
	FailedToGetCluster = "FailedToGetCluster"
	// LocalFailedToGetClusterRoleBindings means that the local controller was unable to retrieve the CRTB-related cluster role bindings to update.
	LocalFailedToGetClusterRoleBindings = "LocalFailedToGetClusterRoleBindings"
	// LocalFailedToGetLabelRequirements indicates local issues with the CRTB meta data preventing creation of label requirements.
	LocalFailedToGetLabelRequirements = "LocalFailedToGetLabelRequirements"
	// FailedToGetNamespace means that the controller was unable to find the project namespace referenced by the CRTB.
	FailedToGetNamespace = "FailedToGetNamespace"
	// FailedToGetRoleBindings means that the controller was unable to retrieve the CRTB-related role bindings to update.
	FailedToGetRoleBindings = "FailedToGetRoleBindings"
	// FailedToGetSubject means that the controller was unable to ensure the User referenced by the CRTB.
	FailedToGetSubject = "FailedToGetSubject"
	// FailedToGrantManagementClusterPrivileges means that the controller was unable to let the CRTB-related RBs grant proper permissions to project-scoped resources.
	FailedToGrantManagementClusterPrivileges = "FailedToGrantManagementClusterPrivileges"
	// FailedToGrantManagementPlanePrivileges means that the controller was unable to authorize the CRTB in the cluster it belongs to.
	FailedToGrantManagementPlanePrivileges = "FailedToGrantManagementPlanePrivileges"
	// LocalFailedToUpdateCRTBLabels means the local controller failed to update the CRTB labels indicating success of CRB/RB label updates.
	LocalFailedToUpdateCRTBLabels = "LocalFailedToUpdateCRTBLabels"
	// LocalFailedToUpdateClusterRoleBindings means that the controller was unable to properly update the CRTB-related cluster role bindings.
	LocalFailedToUpdateClusterRoleBindings = "LocalFailedToUpdateClusterRoleBindings"
	// FailedToUpdateRoleBindings means that the local controller was unable to properly update the CRTB-related role bindings.
	FailedToUpdateRoleBindings = "FailedToUpdateRoleBindings"
	// LocalLabelsSet is a success indicator. The local CRTB-related labels are all set.
	LocalLabelsSet = "LocalLabelsSet"
	// SubjectExists is a success indicator. The CRTB-related subject exists.
	SubjectExists = "SubjectExists"
)

// RemoteConditions is a map listing all remote conditions for filtering
var RemoteConditions = map[string]struct{}{
	RemoteBindingsExist:                     struct{}{},
	RemoteCRTBDeleteOk:                      struct{}{},
	FailedToDeleteClusterRoleBindings:       struct{}{},
	FailedToDeleteSAImpersonator:            struct{}{},
	FailedToEnsureClusterRoleBindings:       struct{}{},
	FailedToEnsureRoles:                     struct{}{},
	FailedToEnsureSAImpersonator:            struct{}{},
	RemoteFailedToGetClusterRoleBindings:    struct{}{},
	RemoteFailedToGetLabelRequirements:      struct{}{},
	FailedToGetRoleTemplate:                 struct{}{},
	FailedToGetRoles:                        struct{}{},
	RemoteFailedToUpdateCRTBLabels:          struct{}{},
	RemoteFailedToUpdateClusterRoleBindings: struct{}{},
	RemoteLabelsSet:                         struct{}{},
}

// LocalConditions is a map listing all remote conditions for filtering
var LocalConditions = map[string]struct{}{
	AuthV2PermissionsOk:                                          struct{}{},
	BadRoleReferences:                                            struct{}{},
	ClusterMembershipBindingForDeleteOk:                          struct{}{},
	FailedClusterMembershipBindingForDelete:                      struct{}{},
	FailedRemovalOfAuthV2Permissions:                             struct{}{},
	FailedRemovalOfMGMTClusterScopedPrivilegesInProjectNamespace: struct{}{},
	FailedToBuildSubject:                                         struct{}{},
	FailedToEnsureClusterMembership:                              struct{}{},
	FailedToGetCluster:                                           struct{}{},
	FailedToGetNamespace:                                         struct{}{},
	FailedToGetRoleBindings:                                      struct{}{},
	FailedToGetSubject:                                           struct{}{},
	FailedToGrantManagementClusterPrivileges:                     struct{}{},
	FailedToGrantManagementPlanePrivileges:                       struct{}{},
	FailedToUpdateRoleBindings:                                   struct{}{},
	LocalBindingsExist:                                           struct{}{},
	LocalCRTBDeleteOk:                                            struct{}{},
	LocalFailedToGetClusterRoleBindings:                          struct{}{},
	LocalFailedToGetLabelRequirements:                            struct{}{},
	LocalFailedToUpdateCRTBLabels:                                struct{}{},
	LocalFailedToUpdateClusterRoleBindings:                       struct{}{},
	LocalLabelsSet:                                               struct{}{},
	SubjectExists:                                                struct{}{},
}

// Successes is a map listing all local __and__ remote success conditions
var Successes = map[string]struct{}{
	AuthV2PermissionsOk:                 struct{}{},
	ClusterMembershipBindingForDeleteOk: struct{}{},
	LocalBindingsExist:                  struct{}{},
	LocalCRTBDeleteOk:                   struct{}{},
	LocalLabelsSet:                      struct{}{},
	RemoteBindingsExist:                 struct{}{},
	RemoteCRTBDeleteOk:                  struct{}{},
	RemoteLabelsSet:                     struct{}{},
	SubjectExists:                       struct{}{},
}
