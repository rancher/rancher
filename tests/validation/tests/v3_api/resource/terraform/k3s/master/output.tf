output "Route53_info" {
  value       = aws_route53_record.aws_route53.*
  description = "List of DNS records"
}

output "masters_public_ip" {
  value = "${aws_instance.master.*.public_ip}"
  description = "The public IP of the AWS node"
}
