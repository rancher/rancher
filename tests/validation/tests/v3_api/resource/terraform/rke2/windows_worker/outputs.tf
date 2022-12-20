output "windows_worker_ips" {
  value = join(",", aws_instance.windows_worker.*.public_ip)
}

output "windows_worker_password_decrypted" {
  value = [
    for agent in aws_instance.windows_worker : rsadecrypt(agent.password_data, file(var.access_key))
  ]
}
