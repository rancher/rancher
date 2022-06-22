output "super_host_fqdn" {
    value = aws_route53_record.super_rancher.name
}

output "hosted1_fqdn" {
    value = aws_route53_record.hosted1_rancher.name
}

output "hosted2_fqdn" {
    value = aws_route53_record.hosted2_rancher.name
}

output "hosted3_fqdn" {
    value = aws_route53_record.hosted3_rancher.name
}