package components

var EKSNodePoolPrefix = `
    node_groups {
	    name = "${var.hostname_prefix}-tf-pool`