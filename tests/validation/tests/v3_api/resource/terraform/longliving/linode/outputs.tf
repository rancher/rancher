output super_host_url {
    value = "https://${module.rancher_hosted.super_host_fqdn}"
}

output hosted1_url {
    value = "https://${module.rancher_hosted.hosted1_fqdn}"
}

output hosted2_url {
    value = "https://${module.rancher_hosted.hosted2_fqdn}"
}

output hosted3_url {
    value = "https://${module.rancher_hosted.hosted3_fqdn}"
}