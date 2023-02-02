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

	if err := creator.add("rancherKubernetesEngine"); err != nil {
		return err
	}

	if err := creator.add("googleKubernetesEngine"); err != nil {
		return err
	}

	if err := creator.add("azureKubernetesService"); err != nil {
		return err
	}

	if err := creator.add("amazonElasticContainerService"); err != nil {
		return err
	}

	if err := creator.addCustomDriver(
		"oraclecontainerengine",
		"https://github.com/rancher-plugins/kontainer-engine-driver-oke/releases/download/v1.8.3/kontainer-engine-driver-oke-linux",
		"7bfde567e6d478f1da8d36531f765d348bff1cd3abe83c70ddf7766f46112170",
		"",
		false,
		"*.oraclecloud.com",
	); err != nil {
		return err
	}
	if err := creator.addCustomDriver(
		"linodekubernetesengine",
		"https://github.com/linode/kontainer-engine-driver-lke/releases/download/v0.0.6/kontainer-engine-driver-lke-linux-amd64",
		"233cbd550a93ded322906b9fc6ebc88b8791e53d31f0d21d501feb0bad77461c",
		"",
		false,
		"api.linode.com",
	); err != nil {
		return err
	}

	return creator.addCustomDriver(
		"opentelekomcloudcontainerengine",
		"https://otc-rancher.obs.eu-de.otc.t-systems.com/cluster/driver/1.0.2/kontainer-engine-driver-otccce_linux_amd64.tar.gz",
		"f2c0a8d1195cd51ae1ccdeb4a8defd2c3147b9a2c7510b091be0c12028740f5f",
		"https://otc-rancher.obs.eu-de.otc.t-systems.com/cluster/ui/v1.1.0/component.js",
		false,
		"*.otc.t-systems.com",
	)
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
