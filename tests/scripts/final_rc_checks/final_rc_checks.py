import requests
import json
import argparse

# Set parser variable to reduce redundancy
parser = argparse.ArgumentParser()

# -fqdn FQDN -api API_Bearer_token
parser.add_argument("-fqdn", "--rancherURL", dest = "fqdn", help="Rancher FQDN, e.g. https://<Rancher FQDN>")
parser.add_argument("-api", "--api", dest = "api_bearer_token", help="API Bearer token")

args = parser.parse_args()

# List of settings to check - URL tags
list_of_urls = ["rke-version", "ui-index", "ui-dashboard-index", "cli-url-linux", 
                "cli-url-darwin", "cli-url-windows", "system-catalog","kdm-branch",
                "ui-k8s-supported-versions-range"]
url_label = ["Released RKE version", "UI Tag", "UI Dashboard Index", "CLI URL (Linux)",
            "CLI URL (Darwin)", "CLI URL (Windows)", "System Chart Catalog", "KDM branch", 
            "UI k8s supported versions range"]
rancher_server_url = args.fqdn

# Disable warnings
requests.packages.urllib3.disable_warnings()

# Add content type and api_bearer_token to the header
headers = {"Content-Type":"application/json","Authorization": "Bearer " + args.api_bearer_token}

# Iterate through list_of_urls to get the values
for url, label in zip(list_of_urls, url_label):
	web_url = rancher_server_url + "/v3/settings/" + url
	response = requests.get(web_url, headers=headers, verify=False)
	if response.status_code == 200:
		print("\t" + label + ": " + response.json()["value"])
	else:
		print("\t" + label + " is not set")