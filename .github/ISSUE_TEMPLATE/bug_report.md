---
name: Bug report
about: Create a report to help us improve
title: ''
labels: ''
assignees: ''

---

<!--
Please search for existing issues first, then read https://rancher.com/docs/rancher/v2.x/en/contributing/#bugs-issues-or-questions to see what we expect in an issue
For security issues, please email security@rancher.com instead of posting a public issue in GitHub. You may (but are not required to) use the GPG key located on Keybase.
-->
**Rancher Server Setup**
- Rancher version (`rancher/rancher`/`rancher/server` image tag or shown bottom left in the UI):
- Installation option (Docker install/Helm Chart):
   - If Helm Chart, Kubernetes Cluster and version (RKE1, RKE2, k3s, EKS, etc):
- Proxy/Cert Details:

**Information about the Cluster**
<!--Local Cluster = Cluster that Rancher is installed on
Downstream = Cluster managed by Rancher
-->
- Cluster Type (Local/Downstream):
<!--Custom = Running a docker command on a node
Imported = Running kubectl apply onto an existing k8s cluster 
Hosted = EKS, GKE, AKS, etc
 Infrastructure Provider = Rancher provisioning the nodes using differnt node drivers (e.g. AWS, Digital Ocean, etc)
-->
  -  If downstream (Custom/Imported or specify provider for Hosted/Infrastructure Provider):
- Kubernetes version (use `kubectl version`):
```
(paste the output here)
```
- If applicable,Docker version (use `docker version`):
```
(paste the output here)
```

**Describe the bug**
<!--A clear and concise description of what the bug is.-->

**To Reproduce**
<!--Steps to reproduce the behavior:
1. Go to '...'
2. Click on '....'
3. Scroll down to '....'
4. See error-->

**Result**

**Expected Result**
<!--A clear and concise description of what you expected to happen.-->

**Screenshots**
<!-- If applicable, add screenshots to help explain your problem.-->



**Additional context**
<!--Add any other context about the problem here.—>
