Examples
========

## Run the controller

`./bin/catalog-controller --config ~/.kube/config`

## Create Custom Resource Definition

`kubectl create -f catalog-crd.yaml`

`kubectl create -f template-crd.yaml`

`kubectl create -f template-version-crd.yaml`

## Create catalog

`kubectl create -f catalog.yaml`

## View templates

`kubectl get templates`

```$xslt
$ kubectl get templates
NAME                                KIND
test-convoy-nfs                     Template.v1.catalog.cattle.io
test-infra-container-crontab        Template.v1.catalog.cattle.io
test-infra-ebs                      Template.v1.catalog.cattle.io
test-infra-ecr                      Template.v1.catalog.cattle.io
test-infra-efs                      Template.v1.catalog.cattle.io
test-infra-healthcheck              Template.v1.catalog.cattle.io
test-infra-ipsec                    Template.v1.catalog.cattle.io
test-infra-k8s                      Template.v1.catalog.cattle.io
test-infra-l2-flat                  Template.v1.catalog.cattle.io
test-infra-netapp-eseries           Template.v1.catalog.cattle.io
test-infra-netapp-ontap-nas         Template.v1.catalog.cattle.io
test-infra-netapp-ontap-san         Template.v1.catalog.cattle.io
test-infra-netapp-solidfire         Template.v1.catalog.cattle.io
test-infra-network-diagnostics      Template.v1.catalog.cattle.io
test-infra-network-policy-manager   Template.v1.catalog.cattle.io
test-infra-network-services         Template.v1.catalog.cattle.io
test-infra-nfs                      Template.v1.catalog.cattle.io
test-infra-per-host-subnet          Template.v1.catalog.cattle.io
test-infra-portworx                 Template.v1.catalog.cattle.io
test-infra-route53                  Template.v1.catalog.cattle.io
test-infra-scheduler                Template.v1.catalog.cattle.io
test-infra-secrets                  Template.v1.catalog.cattle.io
test-infra-vxlan                    Template.v1.catalog.cattle.io
test-infra-windows                  Template.v1.catalog.cattle.io
test-k8s                            Template.v1.catalog.cattle.io
test-kubernetes                     Template.v1.catalog.cattle.io
test-project-cattle                 Template.v1.catalog.cattle.io
test-project-kubernetes             Template.v1.catalog.cattle.io
test-project-mesos                  Template.v1.catalog.cattle.io
test-project-swarm                  Template.v1.catalog.cattle.io
test-project-windows                Template.v1.catalog.cattle.io
test-route53                        Template.v1.catalog.cattle.io
```

`kubectl get templateversions`

```$xslt
kubectl get templateversion                                                                                                                                   master
NAME                                  KIND
test-convoy-nfs-0                     TemplateVersion.v1.catalog.cattle.io
test-convoy-nfs-1                     TemplateVersion.v1.catalog.cattle.io
test-convoy-nfs-2                     TemplateVersion.v1.catalog.cattle.io
test-convoy-nfs-3                     TemplateVersion.v1.catalog.cattle.io
test-infra-container-crontab-0        TemplateVersion.v1.catalog.cattle.io
test-infra-ebs-0                      TemplateVersion.v1.catalog.cattle.io
test-infra-ebs-1                      TemplateVersion.v1.catalog.cattle.io
test-infra-ebs-2                      TemplateVersion.v1.catalog.cattle.io
test-infra-ebs-3                      TemplateVersion.v1.catalog.cattle.io
test-infra-ecr-0                      TemplateVersion.v1.catalog.cattle.io
test-infra-efs-0                      TemplateVersion.v1.catalog.cattle.io
test-infra-efs-1                      TemplateVersion.v1.catalog.cattle.io
test-infra-efs-2                      TemplateVersion.v1.catalog.cattle.io
test-infra-healthcheck-0              TemplateVersion.v1.catalog.cattle.io
test-infra-healthcheck-1              TemplateVersion.v1.catalog.cattle.io
test-infra-healthcheck-2              TemplateVersion.v1.catalog.cattle.io
test-infra-healthcheck-3              TemplateVersion.v1.catalog.cattle.io
test-infra-ipsec-0                    TemplateVersion.v1.catalog.cattle.io
test-infra-ipsec-1                    TemplateVersion.v1.catalog.cattle.io
test-infra-ipsec-2                    TemplateVersion.v1.catalog.cattle.io
test-infra-ipsec-3                    TemplateVersion.v1.catalog.cattle.io
test-infra-ipsec-4                    TemplateVersion.v1.catalog.cattle.io
test-infra-ipsec-5                    TemplateVersion.v1.catalog.cattle.io
test-infra-ipsec-6                    TemplateVersion.v1.catalog.cattle.io
test-infra-ipsec-7                    TemplateVersion.v1.catalog.cattle.io
test-infra-ipsec-8                    TemplateVersion.v1.catalog.cattle.io
test-infra-k8s-0                      TemplateVersion.v1.catalog.cattle.io
test-infra-k8s-1                      TemplateVersion.v1.catalog.cattle.io
test-infra-k8s-10                     TemplateVersion.v1.catalog.cattle.io
test-infra-k8s-11                     TemplateVersion.v1.catalog.cattle.io
```