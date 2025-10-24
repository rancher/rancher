package management

import (
	"fmt"
	"os"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/rancher/rancher/pkg/controllers/management/drivers/kontainerdriver"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func addKontainerDrivers(management *config.ManagementContext) error {
	// create binary drop location if not exists
	err := os.MkdirAll(kontainerdriver.DriverDir, 0777)
	if err != nil {
		return fmt.Errorf("error creating binary drop folder: %v", err)
	}

	creator := driverCreator{
		driversLister: management.Management.KontainerDrivers("").Controller().Lister(),
		drivers:       management.Management.KontainerDrivers(""),
	}

	if err := cleanupImportDriver(creator); err != nil {
		return err
	}

	if err := creator.addCustomDriver(
		"oraclecontainerengine",
		"https://github.com/rancher-plugins/kontainer-engine-driver-oke/releases/download/v1.8.8/kontainer-engine-driver-oke-linux",
		"be98aae12bb4834867dc190aac7078a4caa8236b8ee54bb86215b4f890d83865",
		"",
		false,
		"*.oraclecloud.com",
	); err != nil {
		return err
	}
	if err := creator.addCustomDriver(
		"linodekubernetesengine",
		"https://github.com/linode/kontainer-engine-driver-lke/releases/download/v0.0.13/kontainer-engine-driver-lke-linux-amd64",
		"b17337edeb3b3d4d4f007836e0f9dd946e51eb5cf0945f51f6d48e74123883b5",
		"",
		false,
		"api.linode.com",
	); err != nil {
		return err
	}

	if err := creator.addCustomDriver(
		"opentelekomcloudcontainerengine",
		"https://otc-rancher.obs.eu-de.otc.t-systems.com/cluster/driver/1.1.1/kontainer-engine-driver-otccce_linux_amd64.tar.gz",
		"0998586e1949c826430b10d6b78ee74f2a97769bede0bdd1178c1865a2607065",
		"https://otc-rancher.obs.eu-de.otc.t-systems.com/cluster/ui/v1.2.1/component.js",
		false,
		"*.otc.t-systems.com",
	); err != nil {
		return err
	}

	creator.deleteRKEKontainerDriver()
	creator.deleteBuiltInKontainerDriver("amazonelasticcontainerservice")
	creator.deleteBuiltInKontainerDriver("googlekubernetesengine")
	creator.deleteBuiltInKontainerDriver("azurekubernetesservice")
	creator.deleteKontainerDriver("baiducloudcontainerengine", "https://drivers.rancher.cn")
	creator.deleteKontainerDriver("aliyunkubernetescontainerservice", "https://drivers.rancher.cn")
	creator.deleteKontainerDriver("tencentkubernetesengine", "https://drivers.rancher.cn")
	creator.deleteKontainerDriver("huaweicontainercloudengine", "https://drivers.rancher.cn")

	return nil
}

func cleanupImportDriver(creator driverCreator) error {
	var err error
	if _, err = creator.driversLister.Get("", "import"); err == nil {
		err = creator.drivers.Delete("import", &v1.DeleteOptions{})
	}

	if !errors.IsNotFound(err) {
		return err
	}

	return nil
}

type driverCreator struct {
	driversLister v3.KontainerDriverLister
	drivers       v3.KontainerDriverInterface
}

func (c *driverCreator) add(name string) error {
	logrus.Infof("adding kontainer driver %v", name)

	driver, err := c.driversLister.Get("", name)
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = c.drivers.Create(&v3.KontainerDriver{
				ObjectMeta: v1.ObjectMeta{
					Name:      strings.ToLower(name),
					Namespace: "",
				},
				Spec: v32.KontainerDriverSpec{
					URL:     "",
					BuiltIn: true,
					Active:  true,
				},
				Status: v32.KontainerDriverStatus{
					DisplayName: name,
				},
			})
			if err != nil && !errors.IsAlreadyExists(err) {
				return fmt.Errorf("error creating driver: %v", err)
			}
		} else {
			return fmt.Errorf("error getting driver: %v", err)
		}
	} else {
		driver.Spec.URL = ""

		_, err = c.drivers.Update(driver)
		if err != nil {
			return fmt.Errorf("error updating driver: %v", err)
		}
	}

	return nil
}

func (c *driverCreator) addCustomDriver(name, url, checksum, uiURL string, active bool, domains ...string) error {
	logrus.Infof("adding kontainer driver %v", name)
	_, err := c.driversLister.Get("", name)
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = c.drivers.Create(&v3.KontainerDriver{
				ObjectMeta: v1.ObjectMeta{
					Name: strings.ToLower(name),
				},
				Spec: v32.KontainerDriverSpec{
					URL:              url,
					BuiltIn:          false,
					Active:           active,
					Checksum:         checksum,
					UIURL:            uiURL,
					WhitelistDomains: domains,
				},
				Status: v32.KontainerDriverStatus{
					DisplayName: name,
				},
			})
			if err != nil && !errors.IsAlreadyExists(err) {
				return fmt.Errorf("error creating driver: %v", err)
			}
		} else {
			return fmt.Errorf("error getting driver: %v", err)
		}
	}
	return nil
}

// Delete a deprecated or invalid kontainer driver. Don't return errors to avoid affecting
// Rancher's startup, as the driver will be removed on the next restart.
func (c *driverCreator) deleteKontainerDriver(name, urlPrefix string) {
	driver, err := c.drivers.Get(name, v1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			logrus.Warnf("Error getting kontainer driver %s for deletion: %v", name, err)
		}
		return
	}

	// Don't delete if the driver is active or if the url is not the expected invalid one,
	// as it was likely modified.
	if driver.Spec.Active || !strings.HasPrefix(driver.Spec.URL, urlPrefix) {
		logrus.Infof("Not deleting active or modified kontainer driver %s", name)
		return
	}

	logrus.Infof("Deleting kontainer driver %s", name)
	if err := c.drivers.Delete(name, &v1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		logrus.Warnf("Error deleting node driver %s: %v", name, err)
	}
}

func (c *driverCreator) deleteRKEKontainerDriver() {
	if err := c.drivers.Delete("rancherKubernetesEngine", &v1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		logrus.Warnf("Error deleting rke kontainer driver : %s", err.Error())
	}
}

// deleteBuiltInKontainerDriver deletes the built in kontainer drivers
// Note: even if the drivers are active they are deleted.
func (c *driverCreator) deleteBuiltInKontainerDriver(driverName string) {
	if err := c.drivers.Delete(driverName, &v1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		logrus.Warnf("Error deleting %s kontainer driver : %s", driverName, err.Error())
	}
}
