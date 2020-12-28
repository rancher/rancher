output "worker_ips" {
  value = join(",", aws_instance.worker.*.public_ip)
}
