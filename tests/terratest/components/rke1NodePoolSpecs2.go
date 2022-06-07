package components

var RKE1NodePoolSpecs2 = `"
  hostname_prefix  = var.node_hostname_prefix
  node_template_id = rancher2_node_template.rancher2_node_template.id
  quantity         = `