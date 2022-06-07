package components

var RequiredProviders = `terraform {
  required_providers {
	  rancher2 = {
	    source  = "rancher/rancher2"
	    version = "1.24.0"
	  }
  }
}

`