output "Registration_address" {
  value = "${data.local_file.master_ip.content}"
}

output "master_node_token" {
  value = "${data.local_file.token.content}"
}

output "workers_public_ip" {
  value = "${aws_instance.worker.*.public_ip}"
  description = "The public IP of the AWS node"
}
