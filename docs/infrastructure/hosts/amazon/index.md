---
title: Amazon EC2 Hosts 
layout: default
---

## Adding Amazon Web Services Hosts
---

Before launching a host on AWS, you'll need to find your AWS account credentials as well as your security group information.

### Finding AWS Information using the Console
 
In order to launch an AWS host, you'll need to log in to your AWS management console and find the required security information. We'll walk through the different sections on the host page as well as where to obtain that information from the AWS console.

1. In the **Region** section of the host page, you will need to select a region from AWS. From the navigation bar, select the **region** that you'll want to have your EC2 instances lauched in. Instead of the UI name, you'll need to know the corresponding code name for the region that you selected. Please refer to Amazon EC2 regions [page](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html) to find the most recent code names. 

    ![AWS on Rancher 1]({{site.baseurl}}/img/Rancher_aws1.png)

    Code	|Name   
    -------|------
    ap-northeast-1| Asia Pacific (Tokyo)
    ap-southeast-1 | Asia Pacific (Singapore)
    ap-southeast-2 |Asia Pacific (Sydney)
    eu-central-1 |EU (Frankfurt)
    eu-west-1 | EU (Ireland)
    sa-east-1 | South America (Sao Paulo)
    us-east-1 | US East (N. Virginia)
    us-west-1 | US West (N. California)
    us-west-2 | US West (Oregon)
        
    <br>

    We will determine the **zone** of the region in step 3. 

    **NEED EXAMPLE IMAGE OF REGION ON HOSTS PAGE (no Zone, only Region inputted)**
    ![AWS on Rancher 2]({{site.baseurl}}/img/Rancher_aws2.png)


2. For the **Account Access** information for the hosts page, this is dependent on how you've set up your AWS credentials. If you are the root user, you will be provided with a global **Access Key ID** and **Secret Access Key**, which can be used for all regions. If you have chosen the IAM user route, you will need to find the **Access Key ID** and **Secret Access Key** for the specific region. Refer to Amazon's [documentation](http://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSGettingStartedGuide/AWSCredentials.html) to find the correct keys.

    **Session Token**: If you are using a [temporary set of credentials](http://docs.aws.amazon.com/STS/latest/UsingSTS/Welcome.html), you will need to also get the session token associated with the temporary access key ID and secret access key.

    **NEED EXAMPLE IMAGE OF ACCOUNT ACCESS ON HOSTS PAGE**
    ![AWS on Rancher 2]({{site.baseurl}}/img/Rancher_aws2.png)


3. For the **Networking** section of the hosts, you can choose to use a **VPC** or a **Subnet ID** in the form. In either case, you will need to find the detailed information from the **Subnet**. From the management console, go to the **Networking** section, and select **VPC**. Click on **Subnets** to check the available subnets. Pick the **subnet** that you want to use.

    ![AWS on Rancher 3]({{site.baseurl}}/img/Rancher_aws3.png)

    In the summary tab, check the availability zone. The zone should match the region code, but it will also have an additional character at the end. For example, if you are in `us-west-1`, the availability zone might be `us-west-1a`. The character at the end is the zone that to use in the **Region** section of the host pag.

    **NEED EXAMPLE IMAGE OF REGION ON HOSTS PAGE (adding in Zone)**
    ![AWS on Rancher 4]({{site.baseurl}}/img/Rancher_aws4.png)

    Next, select either the **VPC** or **Subnet ID** to use. This will be used in the networking section of your hosts page.

    **NEED EXAMPLE IMAGE OF NETWORKING ON HOSTS PAGE**
    ![AWS on Rancher 5]({{site.baseurl}}/img/Rancher_aws5.png)

4. We recommend setting a security group up for Rancher and using it to launch EC2 hosts. Even though this is not a required field, you can select a security group to use for launching the EC2 instance. If you don't input a security group, one called _docker-machine_ will be automatically created, but no rules will be created. Therefore, after you have created your instance, you would need to go back into AWS to add the rules to the security group.

    In the same VPC dashboard on AWS, select the **Security Groups** section and click **Create Security Group**. 
    
    ![AWS on Rancher 6]({{site.baseurl}}/img/Rancher_aws6.png)

    A pop-up will appear. Use **rancher** as the group tag, put in a description of the security group and select the **VPC** that is used in the Networking section. If you have chosen to use a **Subnet ID**, make sure to select the same **VPC** as the one in the subnet ID. Click **Yes,Create**. 

    ![AWS on Rancher 7]({{site.baseurl}}/img/Rancher_aws7.png)
    
    Click on the newly created security group and go to the **Inbound Rules** tab. Click on **Edit** to add new rules. 

 * `TCP` Ports `9345` and `9346` are used to display hosts stats and graphs
 * `UDP` Ports `500` and `4500` are used between the hosts for IPsec networking

    | Type | Protocol | Port Range | Source|
    |---|---|---|---|
    Custom TCP Rule | TCP(6) | 9345 | 0.0.0.0/0 |
    Custom TCP Rule | TCP(6) | 9346 | 0.0.0.0/0 |
    Custom UDP Rule | UDP(17) | 500| 0.0.0.0/0 |
    Custom UDP Rule | UDP(17) | 4500| 0.0.0.0/0 |
    
Once you've collected all the  information from the AWS console, let's get ready to deploy our host!

### Launching the EC2 Instance

Provide your host with a name and if desired, give it a description. Fill in the AWS information into the **Account Access**, **Region** and **Networking** section.

The final section to fill out is **Instance** details. The instance can be specified or you can use the defaults provided by [docker machine](http://docs.docker.com/machine/#amazon-web-services). 

**NEED EXAMPLE IMAGE OF INSTANCE SECTION**
![AWS on Rancher 8]({{site.baseurl}}/img/Rancher_aws8.png)

* **AMI**: Select a specific AMI to launch, but remember to select an AMI in your selected region. By default, an Ubuntu 14.04 TLS image in the specific region is specified.
* **Root Size**: Select the root disk size of the instance (in GB). The default is 16. 
* **Instance Type**: Select the instance type from the available [AWS choices](http://aws.amazon.com/ec2/instance-types/). Make sure to use the _Model_ name. The default is t2.micro.
* **IAM**: Select the AWS IAM role name to be used as the instance profile.


Once all the required fields are completed, click **Create** and Rancher will create the EC2 instance and launch the _rancher-agent_ container in the instance. In a minute or two, the host will be active and you are ready to launch services. **ADD LINK TO LAUNCHING SERVICES**


