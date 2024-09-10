# Active Directory GUID -> DN reverse migration utility

**It is recommended to take a snapshot of Rancher before performing this in the event that a restore is required.**


## Critical Notes
* This script will delete and recreate CRTBs/PRTBs/GRBs, which may cause issues with tools (like terraform) which maintain external state.  The original object names are stored in an annotation on the new objects.
* It is recommended to use this script on Rancher v2.7.6 - running this on v2.7.5 may produce performance issues
* This script requires that the Active Directory service account has permissions to read all users known to Rancher.


## Purpose

In order to reverse the effects of migrating Active Directory principalIDs to be based on GUID rather than DN this
utility is required.  It can be run manually via Rancher Agent, or it will automatically run inside Rancher at startup
time if no previous run is detected.  
This utility will:
* Remove any users that were duplicated during the original migration toward GUID-based principalIDs in Rancher 2.7.5
* Update objects that referenced a GUID-based principalID to reference the correct distinguished name based principalID


## Detailed description

This utility will go through all Rancher users and perform an Active Directory lookup using the configured service account to
get the user's distinguished name.  Next, it will perform lookups inside Rancher for all the user's Tokens,
ClusterRoleTemplateBindings, ProjectRoleTemplateBindings, and GlobalRoleBindings.  If any of those objects, including the user object
itself are referencing a principalID based on the GUID of that user, those objects will be updated to reference
the distinguished name-based principalID (unless the utility is run with -dry-run, in that case the only results
are log messages indicating the changes that would be made by a run without that flag).

This utility will also detect and correct the case where a single ActiveDirectory GUID is mapped to multiple Rancher
users.  That condition was likely caused by a race in the original migration to use GUIDs and resulted in a second
Rancher user being created.  This caused Rancher logins to fail for the duplicated user.  The utility remedies
that situation by mapping any tokens and bindings to the original user before removing the newer user, which was
created in error.


## Requirements

A Rancher environment that has Active Directory set up as the authentication provider.  For any environment where
Active Directory is not the authentication provider, this utility will take no action and will exit immediately.


## Usage via Rancher Agent

```bash
./ad-guid-unmigration.sh <AGENT IMAGE> [--dry-run] [--delete-missing]
```
*  The Agent image can be found at: docker.io/rancher/rancher-agent:v2.7.6
*  The --dry-run flag will run the migration utility, but no changes to Rancher data will take place.  The potential changes will be indicated in the log file.
*  The --delete-missing flag will delete Rancher users that can not be found by looking them up in Active Directory. If --dry-run is set, that will prevent users from being deleted regardless of this flag.


## Additional notes
*  The utility will create a configmap named `ad-guid-migration` in the `cattle-system` namespace.  This configmap contains
   a data entry with a key named "ad-guid-migration-status".  If the utility is currently active, that status will be
   set to "Running".  After the utility has completed, the status will be set to "Finished".  If a run is interrupted
   prior to completion, that configmap will retain the status of "Running" and subsequent attempts to run the script will
   immediately exit.  In order to allow it to run again, you can either edit the configmap to remove that key or you can
   delete the configmap entirely.

*  When migrating ClusterRoleTemplateBindings, ProjectRoleTemplateBindings, and GlobalRoleBindings it is necessary to perform the action
   as a delete/create rather than an update.  **This may cause issues if you use tooling that relies on the names of the objects**.
   When a ClusterRoleTemplateBinding or a ProjectRoleTemplateBinding is migrated to a new name, the newly created object
   will contain a label, "ad-guid-previous-name", that will have a value of the name of the object that was deleted.
