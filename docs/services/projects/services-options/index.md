---
title: Services Options
layout: default
---

## Services Configuration
---

As services are created, we simultaneously create a `docker-compose.yml` and `rancher.yml` file of your project. The `docker-compose` yaml file could be used outside of Rancher to start the same set of services using the `docker-compose` commands. Read [here](https://docs.docker.com/compose/) for more information on `docker-compose`. 

The `rancher.yml` file is used to manage the additional information used by Rancher to start services. These fields are not supported inside the docker-compose file.

### Viewing Configurations

In the project dropdown, you can select **View Config** or click on the **file icon**.

<img src="{{site.baseurl}}/img/rancher_services_options_1.png" style="float: left; width: 20%; margin-right: 1%; margin-bottom: 0.5em;">
<img src="{{site.baseurl}}/img/rancher_services_options_2.png" style="float: left; width: 70%; margin-right: 1%; margin-bottom: 0.5em;">
<p style="clear: both;">

### Exporting Configurations

There are a couple of options to export the files. 

Option 1: Download a zip file that contains both files by selecting **Export Config** in the project dropdown menu.

![Services Options on Rancher 3]({{site.baseurl}}/img/rancher_services_options_3.png)

Option 2: Copy the file to your clipboard by clicking on the icon next to the file name that you want to copy. You can copy either the `docker-compose.yml` file or the `rancher-compose.yml` file. 

![Services Options on Rancher 4]({{site.baseurl}}/img/rancher_services_options_4.png)


## Graph View 
---

We can view the project in another view, which shows how all services are related/linked to each other. Clicking on the **graph icon** will show this view.

**NEED IMAGE OF COMPLICATED APPLICATION!**
![Services Options on Rancher 5]({{site.baseurl}}/img/rancher_services_options_5.png)












