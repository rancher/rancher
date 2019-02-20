package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/rancher/rancher/pkg/controllers/management/drivers/kontainerdriver"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
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

	if err := creator.add("import"); err != nil {
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
		"aliyunkubernetescontainerservice",
		"https://github.com/rancher/kontainer-engine-driver-aliyun/releases/download/v0.2.4/kontainer-engine-driver-aliyun-linux",
		"61e8d1a69dae4c9bee7a1618399422300b95436ca747e520329cfc1a724c4180",
		"",
		false,
		"*.aliyuncs.com",
	); err != nil {
		return err
	}

	if err := creator.addCustomDriver(
		"tencentkubernetesengine",
		"https://github.com/rancher/kontainer-engine-driver-tencent/releases/download/v0.2.2/kontainer-engine-driver-tencent-linux",
		"923bde3bcc2201e236b0e6ebcf83ca540dd12d23b5aa4804f12dd37f9beca6c6",
		"",
		false,
		"*.tencentcloudapi.com", "*.qcloud.com",
	); err != nil {
		return err
	}

	if err := creator.addCustomDriver(
		"huaweicontainercloudengine",
		"https://github.com/rancher/kontainer-engine-driver-huawei/releases/download/v0.1.1/kontainer-engine-driver-huawei-linux",
		"8114c33cf166fa8447d3289db5330d38fbe87e09d4130d3d9eb6ba4dd8904a98",
		"",
		false,
		"*.myhuaweicloud.com",
	); err != nil {
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
				Spec: v3.KontainerDriverSpec{
					URL:     "",
					BuiltIn: true,
					Active:  true,
				},
				Status: v3.KontainerDriverStatus{
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
				Spec: v3.KontainerDriverSpec{
					URL:              url,
					BuiltIn:          false,
					Active:           active,
					Checksum:         checksum,
					UIURL:            uiURL,
					WhitelistDomains: domains,
				},
				Status: v3.KontainerDriverStatus{
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
