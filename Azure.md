# Azure Getting Started Guide for Quilt

Quilt is using the Azure Resource Manager (ARM) model. Check out
[this link](https://azure.microsoft.com/en-us/documentation/articles/resource-group-overview/)
for more details.

Quilt interacts with Azure using Azure Service Management API, which uses
role-based access control to grant permitted actions to applications. It
needs to be configured with the credentials needed to generate OAuth tokens
for ARM. To grant Quilt direct access to access and modify your Azure
resources, you must set up an AD (Active Directory) application and a
Service Principal for this application. Here are some brief
characteristics of these two terminologies:

- An AD application is created within the AD
- A Service Principal is created for the application
- Access to the Subscription should be granted to the Service Principal

See the MSDN article on how to set up both: [Use portal to create Active
Directory application that can access resources](https://azure.microsoft.com/en-us/documentation/articles/resource-group-create-service-principal-portal/).
Make sure you set the scope of the role to your subscription.

Following through the tutorial, you have now created an AD application as
well as a service principal for that application. You have assigned the
service principal to a role. Now, you need to give Quilt access to perform
operations. To do that, create a file `credentials.json` in directory
`~/.azure` with the following format:

```
{
  "clientID" : "<Service Principal ID (Client ID in the AD application)>",
  "clientSecret" : "<Service Principal generated key>",
  "subscriptionID" : "<Subscription ID>",
  "tenantID" : "<Azure Active Directory tenant that owns the Service Principal>"
}
```

It is worth noting that permissions around AD don't fall in line exactly
with permissions around the Azure subscription. You might run into issues
such as unable to view or create AD applications. So make sure that you are
a global admin of the directory to which you are part of. This happens often
if you are working on someone else's subscription and the subscription owner
added you as co-administrator in his/her subscription. To check, ask the
subscription owner to check with Azure classic portal, under
Active Directory - directory - Users.
