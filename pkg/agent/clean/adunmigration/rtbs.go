package adunmigration

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3norman "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

// principalsToMigrate collects workunits whose resources we wish to migrate into two groups:
//
//	adWorkUnitsByPrincipal - resources should be migrated to an ActiveDirectory principal with a Distinguished Name
//	duplicateLocalWorkUnitsByPrincipal - resources should be migrated to the local ID of the original (kept) user
func principalsToMigrate(workunits *[]migrateUserWorkUnit) (adWorkUnitsByPrincipal map[string]int, duplicateLocalWorkUnitsByPrincipal map[string]int) {
	// first build a map of guid-principalid -> work unit, which will make the following logic more efficient
	adWorkUnitsByPrincipal = map[string]int{}
	duplicateLocalWorkUnitsByPrincipal = map[string]int{}

	for i, workunit := range *workunits {
		adWorkUnitsByPrincipal[activeDirectoryPrefix+workunit.guid] = i
		for j := range workunit.duplicateUsers {
			duplicateLocalWorkUnitsByPrincipal[activeDirectoryPrefix+workunit.guid] = i
			duplicateLocalWorkUnitsByPrincipal[activeDirectoryPrefix+workunit.distinguishedName] = i
			duplicateLocalWorkUnitsByPrincipal[localPrefix+workunit.duplicateUsers[j].Name] = i
		}
	}

	return adWorkUnitsByPrincipal, duplicateLocalWorkUnitsByPrincipal
}

func collectCRTBs(workunits *[]migrateUserWorkUnit, sc *config.ScaledContext) error {
	crtbInterface := sc.Management.ClusterRoleTemplateBindings("")
	crtbList, err := crtbInterface.List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("[%v] unable to fetch CRTB objects: %v", migrateAdUserOperation, err)
		return err
	}

	adWorkUnitsByPrincipal, duplicateLocalWorkUnitsByPrincipal := principalsToMigrate(workunits)

	for _, crtb := range crtbList.Items {
		if index, exists := adWorkUnitsByPrincipal[crtb.UserPrincipalName]; exists {
			if workUnitContainsName(&(*workunits)[index], crtb.UserName) {
				(*workunits)[index].activeDirectoryCRTBs = append((*workunits)[index].activeDirectoryCRTBs, crtb)
			} else {
				logrus.Warnf("[%v] found CRTB for user with guid-based principal '%v' and name '%v', but no user object with that name matches the GUID or its associated DN. refusing to process",
					identifyAdUserOperation, crtb.UserPrincipalName, crtb.UserName)
			}
		} else if index, exists = duplicateLocalWorkUnitsByPrincipal[crtb.UserPrincipalName]; exists {
			if workUnitContainsName(&(*workunits)[index], crtb.UserName) {
				(*workunits)[index].duplicateLocalCRTBs = append((*workunits)[index].duplicateLocalCRTBs, crtb)
			} else {
				logrus.Warnf("[%v] found CRTB for user with guid-based principal '%v' and name '%v', but no user object with that name matches the GUID or its associated DN. refusing to process",
					identifyAdUserOperation, crtb.UserPrincipalName, crtb.UserName)
			}
		}
	}

	return nil
}

func collectPRTBs(workunits *[]migrateUserWorkUnit, sc *config.ScaledContext) error {
	prtbInterface := sc.Management.ProjectRoleTemplateBindings("")
	prtbList, err := prtbInterface.List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("[%v] unable to fetch PRTB objects: %v", migrateAdUserOperation, err)
		return err
	}

	adWorkUnitsByPrincipal, duplicateLocalWorkUnitsByPrincipal := principalsToMigrate(workunits)

	for _, prtb := range prtbList.Items {
		if index, exists := adWorkUnitsByPrincipal[prtb.UserPrincipalName]; exists {
			if workUnitContainsName(&(*workunits)[index], prtb.UserName) {
				(*workunits)[index].activeDirectoryPRTBs = append((*workunits)[index].activeDirectoryPRTBs, prtb)
			} else {
				logrus.Warnf("[%v] found PRTB for user with guid-based principal '%v' and name '%v', but no user object with that name matches the GUID or its associated DN. refusing to process",
					identifyAdUserOperation, prtb.UserPrincipalName, prtb.UserName)
			}
		} else if index, exists = duplicateLocalWorkUnitsByPrincipal[prtb.UserPrincipalName]; exists {
			if workUnitContainsName(&(*workunits)[index], prtb.UserName) {
				(*workunits)[index].duplicateLocalPRTBs = append((*workunits)[index].duplicateLocalPRTBs, prtb)
			} else {
				logrus.Warnf("[%v] found PRTB for user with guid-based principal '%v' and name '%v', but no user object with that name matches the GUID or its associated DN. refusing to process",
					identifyAdUserOperation, prtb.UserPrincipalName, prtb.UserName)
			}
		}
	}

	return nil
}

func collectGRBs(workunits *[]migrateUserWorkUnit, sc *config.ScaledContext) error {
	grbInterface := sc.Management.GlobalRoleBindings("")
	grbList, err := grbInterface.List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("[%v] unable to fetch GRB objects: %v", migrateAdUserOperation, err)
		return err
	}

	duplicateLocalWorkUnitsByName := map[string]int{}

	for _, workunit := range *workunits {
		for j := range workunit.duplicateUsers {
			duplicateLocalWorkUnitsByName[workunit.duplicateUsers[j].Name] = j
		}
	}

	for _, grb := range grbList.Items {
		if index, exists := duplicateLocalWorkUnitsByName[grb.UserName]; exists {
			(*workunits)[index].duplicateLocalGRBs = append((*workunits)[index].duplicateLocalGRBs, grb)
		}
	}

	return nil
}

func updateCRTB(crtbInterface v3norman.ClusterRoleTemplateBindingInterface, oldCrtb *v3.ClusterRoleTemplateBinding, userName string, principalID string) error {
	newAnnotations := oldCrtb.Annotations
	if newAnnotations == nil {
		newAnnotations = make(map[string]string)
	}
	newAnnotations[adGUIDMigrationAnnotation] = oldCrtb.UserPrincipalName
	newLabels := oldCrtb.Labels
	if newLabels == nil {
		newLabels = make(map[string]string)
	}
	newLabels[migrationPreviousName] = oldCrtb.Name
	newLabels[adGUIDMigrationLabel] = migratedLabelValue
	newCrtb := &v3.ClusterRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:         "",
			Namespace:    oldCrtb.ObjectMeta.Namespace,
			GenerateName: "crtb-",
			Annotations:  newAnnotations,
			Labels:       newLabels,
		},
		ClusterName:       oldCrtb.ClusterName,
		UserName:          userName,
		RoleTemplateName:  oldCrtb.RoleTemplateName,
		UserPrincipalName: principalID,
	}

	// If we get an internal error during any of these ops, there's a good chance the webhook is overwhelmed.
	// We'll take the opportunity to rate limit ourselves and try again a few times.

	backoff := wait.Backoff{
		Duration: 5 * time.Second,
		Factor:   1.1,
		Jitter:   0.1,
		Steps:    10,
	}

	err := wait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		_, err = crtbInterface.Create(newCrtb)
		if err != nil {
			if apierrors.IsInternalError(err) {
				logrus.Errorf("[%v] internal error while creating crtb, will backoff and retry: %v", migrateCrtbsOperation, err)
				return false, err
			}
			return true, fmt.Errorf("[%v] unable to create new CRTB: %w", migrateCrtbsOperation, err)
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("[%v] permanent error when creating crtb, giving up: %v", migrateCrtbsOperation, err)
	}

	err = wait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		err = crtbInterface.DeleteNamespaced(oldCrtb.Namespace, oldCrtb.Name, &metav1.DeleteOptions{})
		if err != nil {
			if apierrors.IsInternalError(err) {
				logrus.Errorf("[%v] internal error while deleting crtb, will backoff and retry: %v", migrateCrtbsOperation, err)
				return false, err
			}
			return true, fmt.Errorf("[%v] unable to delete old CRTB: %w", migrateCrtbsOperation, err)
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("[%v] permanent error when deleting crtb, giving up: %v", migrateCrtbsOperation, err)
	}

	return nil
}

func migrateCRTBs(workunit *migrateUserWorkUnit, sc *config.ScaledContext, dryRun bool) {
	crtbInterface := sc.Management.ClusterRoleTemplateBindings("")
	// First convert all GUID-based CRTBs to their equivalent Distinguished Name variants
	dnPrincipalID := activeDirectoryPrefix + workunit.distinguishedName
	for _, oldCrtb := range workunit.activeDirectoryCRTBs {
		if dryRun {
			logrus.Infof("[%v] DRY RUN: would migrate CRTB '%v' from GUID principal '%v' to DN principal '%v'. "+
				"Annotation, %v, and labels %v and %v would be added, including the name of the previous CRTB instance",
				migrateCrtbsOperation, oldCrtb.Name, oldCrtb.UserPrincipalName, dnPrincipalID, adGUIDMigrationAnnotation, migrationPreviousName, adGUIDMigrationLabel)
		} else {
			err := updateCRTB(crtbInterface, &oldCrtb, workunit.originalUser.Name, dnPrincipalID)
			if err != nil {
				logrus.Errorf("[%v] error while migrating CRTBs for user '%v': %v", migrateCrtbsOperation, workunit.originalUser.Name, err)
			}
		}
	}
	// Now do the same for Local ID bindings on the users we are about to delete, pointing them instead to the merged
	// original user that we will be keeping
	localPrincipalID := localPrefix + workunit.originalUser.Name
	for _, oldCrtb := range workunit.duplicateLocalCRTBs {
		if dryRun {
			logrus.Infof("[%v] DRY RUN: would migrate CRTB '%v' from duplicate local user '%v' to original user '%v'"+
				"Annotation, %v, and labels %v and %v would be added, including the name of the previous CRTB instance",
				migrateCrtbsOperation, oldCrtb.Name, oldCrtb.UserPrincipalName, localPrincipalID, adGUIDMigrationAnnotation, migrationPreviousName, adGUIDMigrationLabel)
		} else {
			err := updateCRTB(crtbInterface, &oldCrtb, workunit.originalUser.Name, localPrincipalID)
			if err != nil {
				logrus.Errorf("[%v] error while migrating crtbs for user '%v': %v", migrateCrtbsOperation, workunit.originalUser.Name, err)
			}
		}
	}
}

func updatePRTB(prtbInterface v3norman.ProjectRoleTemplateBindingInterface, oldPrtb *v3.ProjectRoleTemplateBinding, userName string, principalID string) error {
	newAnnotations := oldPrtb.Annotations
	if newAnnotations == nil {
		newAnnotations = make(map[string]string)
	}
	newAnnotations[adGUIDMigrationAnnotation] = oldPrtb.UserPrincipalName
	newLabels := oldPrtb.Labels
	if newLabels == nil {
		newLabels = make(map[string]string)
	}
	newLabels[migrationPreviousName] = oldPrtb.Name
	newLabels[adGUIDMigrationLabel] = migratedLabelValue
	newPrtb := &v3.ProjectRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:         "",
			Namespace:    oldPrtb.ObjectMeta.Namespace,
			GenerateName: "prtb-",
			Annotations:  newAnnotations,
			Labels:       newLabels,
		},
		ProjectName:       oldPrtb.ProjectName,
		UserName:          userName,
		RoleTemplateName:  oldPrtb.RoleTemplateName,
		UserPrincipalName: principalID,
	}

	// If we get an internal error during any of these ops, there's a good chance the webhook is overwhelmed.
	// We'll take the opportunity to rate limit ourselves and try again a few times.

	backoff := wait.Backoff{
		Duration: 5 * time.Second,
		Factor:   1.1,
		Jitter:   0.1,
		Steps:    10,
	}

	err := wait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		_, err = prtbInterface.Create(newPrtb)
		if err != nil {
			if apierrors.IsInternalError(err) {
				logrus.Errorf("[%v] internal error while creating prtb, will backoff and retry: %v", migratePrtbsOperation, err)
				return false, err
			}
			return true, fmt.Errorf("[%v] unable to create new PRTB: %w", migratePrtbsOperation, err)
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("[%v] permanent error when creating prtb, giving up: %v", migratePrtbsOperation, err)
	}

	err = wait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		err = prtbInterface.DeleteNamespaced(oldPrtb.Namespace, oldPrtb.Name, &metav1.DeleteOptions{})
		if err != nil {
			if apierrors.IsInternalError(err) {
				logrus.Errorf("[%v] internal error while deleting prtb, will backoff and retry: %v", migratePrtbsOperation, err)
				return false, err
			}
			return true, fmt.Errorf("[%v] unable to delete old PRTB: %w", migratePrtbsOperation, err)
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("[%v] permanent error when deleting prtb, giving up: %v", migratePrtbsOperation, err)
	}

	return nil
}

func migratePRTBs(workunit *migrateUserWorkUnit, sc *config.ScaledContext, dryRun bool) {
	prtbInterface := sc.Management.ProjectRoleTemplateBindings("")
	// First convert all GUID-based PRTBs to their equivalent Distinguished Name variants
	dnPrincipalID := activeDirectoryPrefix + workunit.distinguishedName
	for _, oldPrtb := range workunit.activeDirectoryPRTBs {
		if dryRun {
			logrus.Infof("[%v] DRY RUN: would migrate PRTB '%v' from GUID principal '%v' to DN principal '%v'. "+
				"Annotation, %v, and labels %v and %v would be added, including the name of the previous PRTB instance",
				migratePrtbsOperation, oldPrtb.Name, oldPrtb.UserPrincipalName, dnPrincipalID, adGUIDMigrationAnnotation, migrationPreviousName, adGUIDMigrationLabel)

		} else {
			err := updatePRTB(prtbInterface, &oldPrtb, workunit.originalUser.Name, dnPrincipalID)
			if err != nil {
				logrus.Errorf("[%v] error while migrating prtbs for user '%v': %v", migratePrtbsOperation, workunit.originalUser.Name, err)
			}
		}
	}
	// Now do the same for Local ID bindings on the users we are about to delete, pointing them instead to the merged
	// original user that we will be keeping
	localPrincipalID := localPrefix + workunit.originalUser.Name
	for _, oldPrtb := range workunit.duplicateLocalPRTBs {
		if dryRun {
			logrus.Infof("[%v] DRY RUN: would migrate PRTB '%v' from duplicate local user '%v' to original user '%v'. "+
				"Annotation, %v, and labels %v and %v would be added, including the name of the previous PRTB instance",
				migratePrtbsOperation, oldPrtb.Name, oldPrtb.UserPrincipalName, localPrincipalID, adGUIDMigrationAnnotation, migrationPreviousName, adGUIDMigrationLabel)

		} else {
			err := updatePRTB(prtbInterface, &oldPrtb, workunit.originalUser.Name, localPrincipalID)
			if err != nil {
				logrus.Errorf("[%v] error while migrating prtbs for user '%v': %v", migratePrtbsOperation, workunit.originalUser.Name, err)
			}
		}
	}
}

func migrateGRBs(workunit *migrateUserWorkUnit, sc *config.ScaledContext, dryRun bool) {
	grbInterface := sc.Management.GlobalRoleBindings("")

	backoff := wait.Backoff{
		Duration: 5 * time.Second,
		Factor:   1.1,
		Jitter:   0.1,
		Steps:    10,
	}

	for _, oldGrb := range workunit.duplicateLocalGRBs {
		if dryRun {
			logrus.Infof("[%v] DRY RUN: would migrate GRB '%v' from duplicate local user '%v' to original user '%v'. "+
				"Labels %v and %v would be added, including the name of the previous GRB instance",
				migrateGrbsOperation, oldGrb.Name, oldGrb.UserName, workunit.originalUser.Name, migrationPreviousName, adGUIDMigrationLabel)
		} else {
			newLabels := oldGrb.Labels
			if newLabels == nil {
				newLabels = make(map[string]string)
			}
			newLabels[migrationPreviousName] = oldGrb.Name
			newLabels[adGUIDMigrationLabel] = migratedLabelValue

			newGrb := &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:         "",
					GenerateName: "grb-",
					Annotations:  oldGrb.Annotations,
					Labels:       newLabels,
				},
				GlobalRoleName:     oldGrb.GlobalRoleName,
				GroupPrincipalName: oldGrb.GroupPrincipalName,
				UserName:           workunit.originalUser.Name,
			}

			err := wait.ExponentialBackoff(backoff, func() (finished bool, err error) {
				_, err = grbInterface.Create(newGrb)
				if err != nil {
					if apierrors.IsInternalError(err) {
						logrus.Errorf("[%v] internal error while creating GRB, will backoff and retry: %v", migrateGrbsOperation, err)
						return false, err
					}
					return true, fmt.Errorf("[%v] unable to create new GRB: %w", migrateGrbsOperation, err)
				}
				return true, nil
			})
			if err != nil {
				logrus.Errorf("[%v] permanent error while creating GRB, giving up: %v", migrateGrbsOperation, err)
				continue
			}

			err = wait.ExponentialBackoff(backoff, func() (finished bool, err error) {
				err = sc.Management.GlobalRoleBindings("").Delete(oldGrb.Name, &metav1.DeleteOptions{})
				if err != nil {
					if apierrors.IsInternalError(err) {
						logrus.Errorf("[%v] internal error while deleting GRB, will backoff and retry: %v", migrateGrbsOperation, err)
						return false, err
					}
					return true, fmt.Errorf("[%v] unable to delete old GRB: %w", migrateGrbsOperation, err)
				}
				return true, nil
			})
			if err != nil {
				logrus.Errorf("[%v] permanent error when deleting GRB, giving up: %v", migrateGrbsOperation, err)
			}
		}
	}
}
