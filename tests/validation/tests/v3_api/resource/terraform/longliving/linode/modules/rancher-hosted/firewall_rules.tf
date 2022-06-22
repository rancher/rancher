resource "linode_firewall" "load_balancers_firewall" {
  label = "load_balancers_firewall"

  inbound {
    label    = "allow-http"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "80"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-https"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "443"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-ssh"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "22"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound_policy = "DROP"

  outbound_policy = "ACCEPT"

  linodes = [
    linode_instance.super_lb.id,
    linode_instance.hosted1_lb.id,
    linode_instance.hosted2_lb.id
  ]
}

resource "linode_firewall" "rancher_clusters_firewall" {
  label = "rancher_clusters_firewall"

  inbound {
    label    = "allow-http-ingress"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "80"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }
  inbound {
    label    = "allow-https-ingress"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "443"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-ingress-liveness-probe"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "10254"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-rancher-webhook"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "8443, 9433"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-monitoring-metrics"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "9100, 9796"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-liveness-probe"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "9099"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-nodeport-tcp"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "30000-32767"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }
  inbound {
    label    = "allow-nodeport-udp"
    action   = "ACCEPT"
    protocol = "UDP"
    ports    = "30000-32767"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }
  inbound {
    label    = "allow-k8s-api"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "6443"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-k3s-etcd"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "2379, 2380"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-kubelet-metrics"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "10250"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-flannel-vxlan"
    action   = "ACCEPT"
    protocol = "UDP"
    ports    = "8472"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-ssh"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "22"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound_policy = "DROP"

  outbound_policy = "ACCEPT"

  linodes = [
    linode_instance.super_node1.id,
    linode_instance.super_node2.id,
    linode_instance.super_node3.id,
    linode_instance.hosted1_node1.id,
    linode_instance.hosted1_node2.id,
    linode_instance.hosted1_node3.id,
    linode_instance.hosted2_node1.id,
    linode_instance.hosted2_node2.id,
    linode_instance.hosted2_node3.id,
  ]
}

resource "linode_firewall" "rancher_custom_clusters_firewall" {
  label = "rancher_custom_clusters_firewall"

  inbound {
    label    = "allow-http-ingress"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "80"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }
  inbound {
    label    = "allow-https-ingress"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "443"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-ingress-probe"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "10254"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-rancher-webhook"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "8443, 9433"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-canal-flannel-probe"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "9099"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-nodeport-tcp"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "30000-32767"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-nodeport-udp"
    action   = "ACCEPT"
    protocol = "UDP"
    ports    = "30000-32767"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }
  inbound {
    label    = "allow-k8s-api"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "6443"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-k8s-etcd"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "2379, 2380"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-kubelet-metrics"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "10250"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-monitoring-metrics"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "9100, 9796"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-flannel-vxlan"
    action   = "ACCEPT"
    protocol = "UDP"
    ports    = "8472"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound {
    label    = "allow-ssh"
    action   = "ACCEPT"
    protocol = "TCP"
    ports    = "22"
    ipv4     = ["0.0.0.0/0"]
    ipv6     = ["::/0"]
  }

  inbound_policy = "DROP"

  outbound_policy = "ACCEPT"

  linodes = concat(linode_instance.custom_nodes1[*].id, linode_instance.custom_nodes2[*].id)
  
  depends_on = [
    linode_instance.custom_nodes1,
    linode_instance.custom_nodes2
  ]
}
