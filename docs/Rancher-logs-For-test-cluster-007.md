# When run the simulated agent, for cluster 'test-cluster-007'

the rancher side showing 'Waiting' state

## logs from the rancher server when this cluster is registering


2025/08/03 19:34:53 [INFO] [mgmt-cluster-rbac-delete] Creating namespace c-jzl7m
2025/08/03 19:34:53 [INFO] [mgmt-cluster-rbac-delete] Creating Default project for cluster c-jzl7m
2025/08/03 19:34:53 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-jzl7m, err=Operation cannot be fulfilled on namespaces "c-jzl7m": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:34:53 [INFO] [mgmt-project-rbac-create] Creating namespace c-jzl7m-p-jmmbh
2025/08/03 19:34:53 [INFO] [mgmt-cluster-rbac-delete] Creating System project for cluster c-jzl7m
2025/08/03 19:34:53 [INFO] [mgmt-project-rbac-create] Creating creator projectRoleTemplateBinding for user user-8kn8j for project p-jmmbh
2025/08/03 19:34:53 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-jzl7m, err=Operation cannot be fulfilled on namespaces "c-jzl7m": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:34:53 [INFO] [mgmt-project-rbac-create] Creating namespace c-jzl7m-p-8rv6q
2025/08/03 19:34:53 [INFO] [mgmt-cluster-rbac-delete] Updating cluster c-jzl7m
2025/08/03 19:34:53 [INFO] [mgmt-project-rbac-create] Setting InitialRolesPopulated condition on project p-jmmbh
2025/08/03 19:34:53 [INFO] [mgmt-auth-prtb-controller] Creating role/clusterRole p-jmmbh-projectowner
2025/08/03 19:34:53 [INFO] [mgmt-project-rbac-create] Updating project p-jmmbh
2025/08/03 19:34:53 [INFO] [mgmt-project-rbac-create] Creating creator projectRoleTemplateBinding for user user-8kn8j for project p-8rv6q
2025/08/03 19:34:53 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for membership in project p-jmmbh for subject user-8kn8j
2025/08/03 19:34:53 [INFO] [mgmt-cluster-rbac-delete] Creating creator clusterRoleTemplateBinding for user user-8kn8j for cluster c-jzl7m
2025/08/03 19:34:53 [INFO] [mgmt-auth-prtb-controller] Creating role/clusterRole c-jzl7m-clustermember
2025/08/03 19:34:53 [INFO] [mgmt-project-rbac-create] Setting InitialRolesPopulated condition on project p-8rv6q
2025/08/03 19:34:53 [INFO] [mgmt-auth-prtb-controller] Creating role/clusterRole p-8rv6q-projectowner
2025/08/03 19:34:53 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-jzl7m-p-jmmbh, err=Operation cannot be fulfilled on namespaces "c-jzl7m-p-jmmbh": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:34:53 [INFO] [mgmt-project-rbac-create] Updating project p-jmmbh
2025/08/03 19:34:53 [INFO] [mgmt-auth-prtb-controller] Creating clusterRoleBinding for membership in cluster c-jzl7m for subject user-8kn8j
2025/08/03 19:34:53 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for membership in project p-8rv6q for subject user-8kn8j
2025/08/03 19:34:53 [INFO] [mgmt-auth-prtb-controller] Creating role project-owner in namespace c-jzl7m-p-jmmbh
2025/08/03 19:34:53 [INFO] [mgmt-project-rbac-create] Updating project p-8rv6q
2025/08/03 19:34:53 [INFO] [mgmt-auth-prtb-controller] Updating clusterRoleBinding crb-sshulypplw for cluster membership in cluster c-jzl7m for subject user-8kn8j
2025/08/03 19:34:53 [INFO] [mgmt-auth-prtb-controller] Creating role admin in namespace c-jzl7m-p-jmmbh
2025/08/03 19:34:53 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-jzl7m-p-jmmbh, err=Operation cannot be fulfilled on namespaces "c-jzl7m-p-jmmbh": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:34:53 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for subject user-8kn8j with role project-owner in namespace c-jzl7m-p-jmmbh
2025/08/03 19:34:53 [INFO] [mgmt-auth-crtb-controller] Creating role/clusterRole c-jzl7m-clusterowner
2025/08/03 19:34:53 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for subject user-8kn8j with role admin in namespace c-jzl7m-p-jmmbh
2025/08/03 19:34:53 [INFO] [mgmt-cluster-rbac-delete] Setting InitialRolesPopulated condition on cluster c-jzl7m
2025/08/03 19:34:53 [INFO] [mgmt-cluster-rbac-delete] Updating cluster c-jzl7m
2025/08/03 19:34:53 [INFO] [mgmt-auth-crtb-controller] Creating clusterRoleBinding for membership in cluster c-jzl7m for subject user-8kn8j
2025/08/03 19:34:53 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-jzl7m-p-8rv6q, err=Operation cannot be fulfilled on namespaces "c-jzl7m-p-8rv6q": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:34:53 [INFO] [mgmt-auth-prtb-controller] Updating clusterRoleBinding crb-sshulypplw for cluster membership in cluster c-jzl7m for subject user-8kn8j
2025/08/03 19:34:53 [INFO] [mgmt-auth-crtb-controller] Creating role cluster-owner in namespace c-jzl7m
2025/08/03 19:34:53 [INFO] [mgmt-auth-prtb-controller] Creating role project-owner in namespace c-jzl7m-p-8rv6q
2025/08/03 19:34:53 [INFO] [mgmt-project-rbac-create] Updating project p-8rv6q
2025/08/03 19:34:53 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject user-8kn8j with role cluster-owner in namespace c-jzl7m
2025/08/03 19:34:53 [INFO] [mgmt-auth-crtb-controller] Creating role cluster-owner in namespace c-jzl7m-p-8rv6q
2025/08/03 19:34:53 [INFO] [mgmt-auth-prtb-controller] Creating role admin in namespace c-jzl7m-p-8rv6q
2025/08/03 19:34:53 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-jzl7m-p-8rv6q, err=Operation cannot be fulfilled on namespaces "c-jzl7m-p-8rv6q": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:34:53 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject user-8kn8j with role cluster-owner in namespace c-jzl7m-p-8rv6q
2025/08/03 19:34:53 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for subject user-8kn8j with role admin in namespace c-jzl7m-p-8rv6q
2025/08/03 19:34:53 [INFO] [mgmt-auth-crtb-controller] Creating role cluster-owner in namespace c-jzl7m-p-jmmbh
2025/08/03 19:34:53 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for subject user-8kn8j with role project-owner in namespace c-jzl7m-p-8rv6q
2025/08/03 19:34:53 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject user-8kn8j with role cluster-owner in namespace c-jzl7m-p-jmmbh
2025/08/03 19:34:54 [INFO] [mgmt-cluster-rbac-delete] Updating cluster c-jzl7m
2025/08/03 19:34:58 [INFO] Handling backend connection request [c-jzl7m]
2025/08/03 19:35:13 [ERROR] error syncing 'c-jzl7m': handler cluster-deploy: cannot connect to the cluster's Kubernetes API, requeuing
2025/08/03 19:35:28 [ERROR] error syncing 'c-jzl7m': handler cluster-deploy: cannot connect to the cluster's Kubernetes API, requeuing
2025/08/03 19:35:38 [INFO] Stopping cluster agent for c-jzl7m
2025/08/03 19:35:38 [ERROR] failed to start cluster controllers c-jzl7m: context canceled
2025/08/03 19:35:43 [ERROR] error syncing 'c-jzl7m': handler cluster-deploy: cannot connect to the cluster's Kubernetes API, requeuing
2025/08/03 19:35:58 [ERROR] error syncing 'c-jzl7m': handler cluster-deploy: cannot connect to the cluster's Kubernetes API, requeuing
2025/08/03 19:36:13 [ERROR] error syncing 'c-jzl7m': handler cluster-deploy: cannot connect to the cluster's Kubernetes API, requeuing
2025/08/03 19:36:23 [INFO] Stopping cluster agent for c-jzl7m
2025/08/03 19:36:23 [ERROR] failed to start cluster controllers c-jzl7m: context canceled
2025/08/03 19:36:28 [ERROR] error syncing 'c-jzl7m': handler cluster-deploy: cannot connect to the cluster's Kubernetes API, requeuing
^Ccontext canceled
