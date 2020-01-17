# Rancher

[![Build Status](https://drone-publish.rancher.io/api/badges/rancher/rancher/status.svg?branch=master)](https://drone-publish.rancher.io/rancher/rancher)
[![Docker Pulls](https://img.shields.io/docker/pulls/rancher/rancher.svg)](https://store.docker.com/community/images/rancher/rancher)
[![Go Report Card](https://goreportcard.com/badge/github.com/rancher/rancher)](https://goreportcard.com/report/github.com/rancher/rancher)

Rancher is an open source project that provides a container management platform built for organizations that deploy containers in production. Rancher makes it easy to run Kubernetes everywhere, meet IT requirements, and empower DevOps teams.

> Looking for Rancher 1.6.x info?  [Click here](https://github.com/rancher/rancher/blob/master/README_1_6.md)

## Latest Release

* Latest - v2.3.4 - `rancher/rancher:latest` - Read the full release [notes](https://github.com/rancher/rancher/releases/tag/v2.3.4).

* Stable - v2.3.4 - `rancher/rancher:stable` - Read the full release [notes](https://github.com/rancher/rancher/releases/tag/v2.3.4).

To get automated notifications of our latest release, you can watch the announcements category in our [forums](http://forums.rancher.com/c/announcements), or subscribe to the RSS feed `https://forums.rancher.com/c/announcements.rss`.

## Quick Start

    sudo docker run -d --restart=unless-stopped -p 80:80 -p 443:443 rancher/rancher

Open your browser to https://localhost

## Installation
Rancher can be deployed in either a single node or multi-node setup.  Please refer to the following for guides on how to get Rancher up and running.

* [Single Node Install](https://rancher.com/docs/rancher/v2.x/en/installation/single-node/)
* [High Availability (HA) Install](https://rancher.com/docs/rancher/v2.x/en/installation/ha/)

> **No internet access?**  Refer to our [Air Gap Installation](https://rancher.com/docs/rancher/v2.x/en/installation/air-gap-installation/) for instructions on how to use your own private registry to install Rancher.

### Minimum Requirements

* Operating Systems
  * Ubuntu 16.04 (64-bit)
  * Red Hat Enterprise Linux 7.5 (64-bit)
  * RancherOS 1.4 (64-bit)
* Hardware
  * 4 GB of Memory
* Software
  * Docker v1.12.6, 1.13.1, 17.03.2

### Using Rancher

To learn more about using Rancher, please refer to our [Rancher Documentation](https://rancher.com/docs/rancher/v2.x/en/).

## Source Code

This repo is a meta-repo used for packaging and contains the majority of rancher codebase.  Rancher does include other Rancher projects  including:
* https://github.com/rancher/types
* https://github.com/rancher/norman
* https://github.com/rancher/kontainer-engine
* https://github.com/rancher/rke
* https://github.com/rancher/ui

Rancher also includes other open source libraries and projects, [see go.mod](https://github.com/rancher/rancher/blob/master/go.mod) for the full list.

## Support, Discussion, and Community
If you need any help with Rancher or RancherOS, please join us at either our [Rancher forums](http://forums.rancher.com/), [#rancher IRC channel](http://webchat.freenode.net/?channels=rancher) or [Slack](https://slack.rancher.io/) where most of our team hangs out at.

Please submit any **Rancher** bugs, issues, and feature requests to [rancher/rancher](//github.com/rancher/rancher/issues). 

Please submit any **RancherOS** bugs, issues, and feature requests to [rancher/os](//github.com/rancher/os/issues).

For security issues, please email security@rancher.com instead of posting a public issue in GitHub.  You may (but are not required to) use the GPG key located on [Keybase](https://keybase.io/rancher).

# License

Copyright (c) 2014-2019 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
