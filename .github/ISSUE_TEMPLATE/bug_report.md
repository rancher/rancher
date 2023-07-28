---
name: Bug report
about: Create a report to help us improve
title: "[BUG]"
labels: kind/bug
assignees: ''

---

**Rancher Server Setup**
- Rancher version:
- Installation option (Docker install/Helm Chart):
   - If Helm Chart, Kubernetes Cluster and version (RKE1, RKE2, k3s, EKS, etc):
- Proxy/Cert Details:

**Information about the Cluster**
- Kubernetes version:
- Cluster Type (Local/Downstream):
   - If downstream, what type of cluster? (Custom/Imported or specify provider for Hosted/Infrastructure Provider):
<!--
* Custom = Running a docker command on a node
* Imported = Running kubectl apply onto an existing k8s cluster
* Hosted = EKS, GKE, AKS, etc
 * Infrastructure Provider = Rancher provisioning the nodes using different node drivers (e.g. AWS, Digital Ocean, etc)
-->

**User Information**
- What is the role of the user logged in? (Admin/Cluster Owner/Cluster Member/Project Owner/Project Member/Custom)
  - If custom, define the set of permissions:



**Describe the bug**
<!--A clear and concise description of what the bug is.-->

**To Reproduce**
<!--Steps to reproduce the behavior-->

**Result**

**Expected Result**
<!--A clear and concise description of what you expected to happen.-->

**Screenshots**
<!-- If applicable, add screenshots to help explain your problem.-->

**Additional context**
<!--Add any other context about the problem here.-->
