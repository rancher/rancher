# Rancher

*This file is auto-generated from README-template.md, please make any changes there.*

[![Build Status](https://drone-publish.rancher.io/api/badges/rancher/rancher/status.svg?branch=release/v2.7)](https://drone-publish.rancher.io/rancher/rancher)
[![Docker Pulls](https://img.shields.io/docker/pulls/rancher/rancher.svg)](https://store.docker.com/community/images/rancher/rancher)
[![Go Report Card](https://goreportcard.com/badge/github.com/rancher/rancher)](https://goreportcard.com/report/github.com/rancher/rancher)

Rancher is an open source container management platform built for organizations that deploy containers in production. Rancher makes it easy to run Kubernetes everywhere, meet IT requirements, and empower DevOps teams.

> Looking for Rancher 1.6.x info? [Click here](https://github.com/rancher/rancher/blob/master/README_1_6.md)

## Quick Start

    sudo docker run -d --restart=unless-stopped -p 80:80 -p 443:443 --privileged rancher/rancher

Open your browser to https://localhost

## Installation

See [Installing/Upgrading Rancher](https://rancher.com/docs/rancher/v2.6/en/installation/) for all installation options.

### Minimum Requirements

* Operating Systems
  * Please see [Support Matrix](https://rancher.com/support-matrix/) for specific OS versions for each Rancher version. Note that the link will default to the support matrix for the latest version of Rancher. Use the left navigation menu to select a different Rancher version. 
* Hardware & Software
  * Please see [Installation Requirements](https://rancher.com/docs/rancher/v2.6/en/installation/requirements/) for hardware and software requirements.

### Using Rancher

To learn more about using Rancher, please refer to our [Rancher Documentation](https://rancher.com/docs/rancher/v2.6/en/).

## Source Code

This repo is a meta-repo used for packaging and contains the majority of Rancher codebase. For other Rancher projects and modules, [see go.mod](https://github.com/rancher/rancher/blob/release/v2.6/go.mod) for the full list.

Rancher also includes other open source libraries and projects, [see go.mod](https://github.com/rancher/rancher/blob/release/v2.6/go.mod) for the full list.

## Support, Discussion, and Community
If you need any help with Rancher, please join us at either our [Rancher forums](http://forums.rancher.com/) or [Slack](https://slack.rancher.io/) where most of our team hangs out at.

Please submit any Rancher bugs, issues, and feature requests to [rancher/rancher](https://github.com/rancher/rancher/issues).

For security issues, please first check our [security policy](SECURITY.md) and email security-rancher@suse.com instead of posting a public issue in GitHub.  You may (but are not required to) use the GPG key located on [Keybase](https://keybase.io/rancher).

# License

Copyright (c) 2014-2023 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
