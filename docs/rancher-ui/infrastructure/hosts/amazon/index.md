---
title: Amazon EC2 Hosts 
layout: default
---

## Adding Amazon Web Services Hosts
---

Rancher supports provisioning [Amazon EC2](http://aws.amazon.com/ec2/) hosts using `docker machine`. 

### Finding AWS Credentials
Before launching a host on AWS, you'll need to find your AWS account credentials as well as your security group information. The **Account Access** information can be found using Amazon's [documentation](http://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSGettingStartedGuide/AWSCredentials.html) to find the correct keys. When creating an **access key** and **secret key**, please be sure to save it somewhere as it will not be available unless you create a new key pair. 

### Launching Amazon EC2 Host(s)

Under the Infrastructure -> Hosts tab, click **Add Host**. Select the **Amazon EC2** icon. Provide your AWS **Access key** and **Secret Key**, click on **Next: Authenticate & select a network**. Rancher will use your credentials to determine what is available in AWS to launch instances.

![AWS on Rancher 1]({{site.baseurl}}/img/rancher_aws_1.png)

You'll need to select the available region and zone to create the instance. Depending on which region/zone that you select, the available VPC IDs and Subnet IDs will be displayed. Select a **VPC ID** or **Subnet ID**, and click on **Next: Select a Security Group**. 

![AWS on Rancher 2]({{site.baseurl}}/img/rancher_aws_2.png)

Next, you'll select a security group to use for the hosts. There are two choices for security groups. The **Standard** option will create or use the existing `rancher-machine` security group. If Rancher creates the `rancher-machine` security group, it will open up all the necessary ports to allow Rancher to work successfully. `docker machine` will automatically open up port `2376`, which is the Docker daemon port. 

In the **Custom** option, you can choose an existing security group, but you will need to ensure that specific ports are open in order for Rancher to be working correctly. 

<a id="EC2Ports"></a>
### Required Ports for Rancher to work:

 * From the rancher server to TCP port `22` (SSH to install and configure Docker)
 * From and To all other hosts on UDP ports `500` and `4500` (for IPsec networking)

As of our Beta release (v0.24.0), we no longer require any additional TCP ports. But if you are using a version prior to Beta, then you will need to add the following ports:

* From the internet to TCP ports `9345` and `9346` (for UI hosts stats/graphs) 

> **Note:** If you re-use the `rancher-machine` security group, any missing ports in the security group will not be re-opened. You will need to check the security group in AWS if the host does not launch correctly. 

After choosing your security option, click on **Next: Set Instance Options**. 

![AWS on Rancher 3]({{site.baseurl}}/img/rancher_aws_3.png)

Finally, you'll just need to finish filling out the final details of the host(s).

1. Select the number of hosts you want to launch using the slider.
2. Provide a **Name** and if desired, **Description** for the host.
3. Select the **Instance Type** that you want launched. 
4. Select the **Root Size** of the image. The default in `docker machine` is 16GB, which is what we have defaulted in Rancher. 
5. (Optional) For the **AMI**, `docker machine` defaults with an Ubuntu 14.04 TLS image in the specific region. You also have the option to select your own AMI. If you input your own AMI, make sure it's available in that region!
6. (Optional) Provide the **IAM Profile** to be used as an instance profile. 
7. (Optional) Add **[labels]({{site.baseurl}}/docs/rancher-ui/infrastructure/hosts/#labels)** to hosts to help organize your hosts and to [schedule services]({{site.baseurl}}/docs/rancher-ui/applications/stacks/adding-services/#scheduling-services).
8. When complete, click **Create**. 

Rancher will create the EC2 instance(s) and launch the _rancher-agent_ container in the instance. In a couple of minutes, the host will be active and available for [services]({{site.baseurl}}/docs/rancher-ui/applications/stacks/adding-services/).


