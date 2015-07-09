---
title: API & Keys
layout: default
---

## API & Keys
---

The API endpoint can be found by going to the account settings dropdown menu and clicking on **API & Keys**. The endpoint provided will direct you to the specific project that you are in. 

If [access control]({{site.baseurl}}/docs/configuration/access-control/) is not configured, anyone with the IP address will have access to your Rancher's API. It's highly recommended to enable access control.

Once access control is enabled, an API key will need to be created for each [project]({{site.baseurl}}/docs/configuration/project) in order to access the API for the specific project. 

Within Rancher, all objects can be viewed in the API by selecting the **View in API** option in the object's dropdown menu.

### Adding API Keys

Before adding any API Keys, please confirm that you are in the correct project. Each API Key is project specific. Click on **Add API Key**. Rancher will generate and display your API Key for your project. 

Provide a **Name** for the API Key and click on **Save**. 

### Using API Keys

When access control is configured and you go to the API endpoint, you will be prompted for your API key. The username is the access key and the password is the secret key. 

### Editing API Keys

All options for an API key are accessible through the dropdown menu on the right hand side of the listed key.

For any _Active_ key, you can **Deactivate** the key, which would prohibit the use of those credentials. The key will be labeled in an _Inactive_ state.

For any _Inactive_ key, you have two options. You can **Activate** the key, which will allow the credentials to access the API again. Alternatively, you can **Delete** the key, which will remove the credentials from the project.

You can **Edit** any key, which allows you to change the name and description of the API key. You will not be able to change the actual access key or secret key. If you want a new key pair, you'll need to add a new one.