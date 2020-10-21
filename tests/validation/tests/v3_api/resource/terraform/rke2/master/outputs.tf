output "Route53_info" {
  value       = aws_route53_record.aws_route53.*
  description = "List of DNS records"
}

output "kubeconfig" {
  value = var.resource_name
  description = "kubeconfig of the cluster created"
}

output "master_ips" {
  value = join("," , aws_instance.master.*.public_ip,aws_instance.master2.*.public_ip)
}