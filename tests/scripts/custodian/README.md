# QA Cloud Custodian

QA custodian will clean up resources in AWS, GCR, and AZURE. Currently, we have automation configured for:
* AWS
  * instances
  * NLBs
  * EKS
* AZURE
  * VMs
  * AKS
* GCP
  * instances

### Getting Started
New Hires - make a PR adding one line to the `user-keys.txt` file. Use that key in ALL resources you create when using SUSE resources.  Keep it short (6 characters or less) unique to you, and recognizable by others (And, this is case sensitive!). For example, if your name was Jane Elaine Morrison, something like `jem` would suffice. Just make sure you can remember it, and others can know who's it is if they are on your team. 

### About the Code
This suite is fairly simple, and runs using [Cloud Custodian](https://cloud-custodian.github.io/cloud-custodian/docs/quickstart/index.html). Our modifications consist of the following:
* text files
  * `*-keys.txt` representing special keys that correspond with QA's resources
  * `regions.txt` (AWS explicit) representing the regions we use on a regular basis, and therefore what the custodian will check against 
* `.yaml` files, which are different configurations for the custodian to use when running. 