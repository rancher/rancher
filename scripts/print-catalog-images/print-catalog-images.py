#!/usr/bin/env python2.7

# Requires git, python 2.7 and python modules 'PyYAML' and 'semver'
#
# apt-get install git python-pip
# pip install -r requirements.txt

import argparse
import collections
import json
import os
import re
import semver
import sets
import subprocess
import sys
import uuid
import yaml

description = """Computes Docker images required to run each infrastructure service for a
specific Rancher version. This is valuable when preparing an air-gapped Rancher
installation for the latest infrastructure services without legacy image bloat.
"""

parser = argparse.ArgumentParser(description=description)

parser.add_argument('-u', '--url',
                    default='https://git.rancher.io/rancher-catalog',
                    help='Rancher catalog URL accessible in airgap environment')
parser.add_argument('-b', '--branch',
                    help='Rancher catalog branch accessible in airgap environment')
parser.add_argument('-k', '--k8saddons', action='store_true',
                    help='Print k8s addon images')
parser.add_argument('-ku', '--k8surl',
                    default='https://github.com/rancher/kubernetes-package',
                    help='Rancher URL for kubernetes-package accessible in airgap environment')
parser.add_argument('-v', '--version',
                    required=True,
                    help='Rancher Server version')
parser.add_argument('-j', '--json', action='store_true',
                    help='Print JSON output')
args = parser.parse_args()


def get_catalog_branch(version):
    if semver.match(version, "<=1.6.0"):
        return "master"
    elif semver.match(version, ">1.6.0") and semver.match(version, "<2.0.0"):
        return "v1.6-release"
    elif semver.match(version, ">=2.0.0"):
        return "v2.0-release"
    else:
        print "Unknown version"
        sys.exit(1)


def print_keys(header, iter):
    temp = header
    for key in iter.iterkeys():
        temp += " " + key
    print temp


def optimal_version_dir(rancher_version, service_dir):
    # Parse each version dir's rancher-compose.yml
    version_dirs = {}
    for service_version_dir in os.listdir(service_dir):
        version_dir = service_dir + "/" + service_version_dir
        if os.path.isdir(version_dir):
            rancher_compose_filepath = version_dir + "/rancher-compose.yml"
            if os.path.isfile(rancher_compose_filepath):
                try:
                    with file(rancher_compose_filepath, 'r') as f:
                        rancher_compose = yaml.load(f)
                        version_dirs[service_version_dir] = rancher_compose
                except yaml.YAMLError, exc:
                    print "Error in rancher-compose.yml file: ", exc
            else:
                print version_dir + ": missing rancher-compose.yml"
    # print_keys("Unfiltered:", version_dirs)

    # Filter version dirs by min/max rancher version
    filtered = {}
    for key, value in version_dirs.iteritems():
        if '.catalog' in value:
            catalog = value['.catalog']
            if 'minimum_rancher_version' in catalog:
                min_version = catalog['minimum_rancher_version'].lstrip('v')
                if semver.compare(rancher_version, min_version) < 0:
                    continue
            if 'maximum_rancher_version' in catalog:
                max_version = catalog['maximum_rancher_version'].lstrip('v')
                if semver.compare(rancher_version, max_version) > 0:
                    continue
        filtered[key] = value
    # print_keys("Server Version:", filtered)

    # Bail out if only one remains
    if len(filtered) == 1:
        for key, value in filtered.iteritems():
            return key, value['.catalog']['version']
        return list(filtered)[0]

    # Try to return the template version in config.yml
    try:
        template_config = yaml.load(file(service_dir + "/config.yml", 'r'))
        if 'version' in template_config:
            version = template_config['version']
            for key, value in filtered.iteritems():
                if '.catalog' in value:
                    catalog = value['.catalog']
                    if 'version' in catalog and catalog['version'] == version:
                        return key, value['.catalog']['version']
    except yaml.YAMLError, exc:
        if args.json == False:
            print "Error in config.yml file: ", exc

    # Choose the highest ordinal value
    maxkey = -1
    for key in filtered.iterkeys():
        try:
            keyint = int(key)
            if keyint > maxkey:
                maxkey = keyint
        except:
            pass
    if maxkey > -1:
        return str(maxkey), filtered[str(maxkey)]['.catalog']['version']
    else:
        return "", ""

def version_pod_images(k8s_addons_dir):
    images = sets.Set()
    if os.path.isfile(k8s_addons_dir + "/addon-templates/README.md"):
        k8saddonsreadme = open(k8s_addons_dir + "/addon-templates/README.md", 'r').read().splitlines()
        for line in k8saddonsreadme:
            match = re.search("^\s+-\s+(.*?:(v|\d+).*)$", line)
            if match:
                image = match.group(1)
                images.add(image)
    else:
        for root, dirs, files in os.walk(k8s_addons_dir + "/addon-templates"):
            for f in files:
                fullpath = os.path.join(root, f)
                if fullpath.endswith(".yaml") or fullpath.endswith(".yaml.sed"):
                    with open(fullpath, 'r') as f:
                        filedata = f.read()
                        for line in filedata.splitlines():
                            match = re.search("image: (.*?)$", line)
                            if match:
                                image = match.group(1)
                                image = image.replace('"','')
                                images.add(image)
    return images

def version_images(service_version_dir):
    images = sets.Set()
    compose_filepath = service_version_dir + "/docker-compose.yml"
    compose_tpl_filepath = service_version_dir + "/docker-compose.yml.tpl"

    filedata = ''
    templatevalues = {}
    if os.path.isfile(compose_tpl_filepath):
        with open(compose_tpl_filepath, 'r') as f:
            filedata = f.read()
            for line in filedata.splitlines():
                  match = re.search( r'(\$.*?):="(.*?)"', line)
                  if match:
                      key, value = match.groups()
		      templatevalues[key] = value
            for k, v in templatevalues.iteritems():
                filedata = re.sub(r'{{' + re.escape(k) + '}}', v, filedata)
            filedata, subs = re.subn('{{[^}]*}}', '', filedata)
    elif os.path.isfile(compose_filepath):
        with open(compose_filepath, 'r') as f:
            filedata = f.read()
    else:
        print "missing docker-compose.yml[.tpl]"
        return images

    try:
        docker_compose = yaml.load(filedata)
        # handle v1/v2 docker-compose
        services = docker_compose
        if 'services' in services:
            services = docker_compose['services']

        for serviceName in services:
            service = services[serviceName]
            if 'image' in service:
                images.add(service['image'])

    except yaml.YAMLError, exc:
        if args.json == False:
            print "Error in docker-compose.yml file: ", exc

    return images


version = args.version.lstrip('v')
if args.branch is None:
    args.branch = get_catalog_branch(version)

if args.json == False:
    print 'Rancher Version: ' + version
    print 'Catalog URL: ' + args.url
    print 'Catalog Branch: ' + args.branch
    print

catalog_dir = str(uuid.uuid4())
try:
    subprocess.check_call(["git", "clone", args.url,
                           "--quiet", "--single-branch", "--branch", args.branch,
                           catalog_dir])
except subprocess.CalledProcessError:
    sys.exit(1)


jsondata = collections.defaultdict(list)
jsondata['services'] = {}
infra_dir = catalog_dir + "/infra-templates"
for infra_service in os.listdir(infra_dir):
    service_dir = infra_dir + "/" + infra_service
    if os.path.isdir(service_dir):
        version_dir, template_ver = optimal_version_dir(version, service_dir)
        if version_dir != "":
            if args.json == True:
                jsondata['services'][infra_service] = {}
                jsondata['services'][infra_service]['version'] = template_ver
                jsondata['services'][infra_service]['images'] = list()
            else:
                print infra_service + ": " + template_ver
            for image in version_images(service_dir + "/" + version_dir):
                if args.json == True:
                    jsondata['services'][infra_service]['images'].append(image)
                else:
                    print "    - " + image
            if infra_service == "k8s":
                if args.k8saddons == True:
                    k8s_addons_dir = str(uuid.uuid4())
                    try:
                        subprocess.check_call(["git", "clone", args.k8surl,
                           "--quiet", "--single-branch", "-c", "advice.detachedHead=false", "--branch", template_ver,
                           k8s_addons_dir])
                    except subprocess.CalledProcessError:
                        sys.exit(1)
                    for image in version_pod_images(k8s_addons_dir):
                        if args.json == True:
                            jsondata['services'][infra_service]['images'].append(image)
                        else:
                            print "    - " + image

if args.json == True:
    print json.dumps(jsondata)

subprocess.call(["rm", "-rf", catalog_dir])
if args.k8saddons == True:
    subprocess.call(["rm", "-rf", k8s_addons_dir])
