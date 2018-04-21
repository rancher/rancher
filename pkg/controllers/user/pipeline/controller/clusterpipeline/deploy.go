package clusterpipeline

import (
	"fmt"

	"github.com/rancher/rancher/pkg/controllers/user/pipeline/engine/jenkins"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/randomtoken"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getSecret() *corev1.Secret {
	token, err := randomtoken.Generate()
	if err != nil {
		logrus.Warningf("warning generate random token got - %v, use default instead", err)
		token = jenkins.JenkinsDefaultToken
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.PipelineNamespace,
			Name:      "jenkins",
		},
		Data: map[string][]byte{
			"jenkins-admin-password": []byte(token),
			"jenkins-admin-user":     []byte(jenkins.JenkinsDefaultUser),
		},
	}
}

func getJenkinsService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.PipelineNamespace,
			Name:      "jenkins",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "jenkins",
			},
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 8080,
				},
				{
					Name: "agent",
					Port: 50000,
				},
			},
		},
	}
}

func getConfigMap() *corev1.ConfigMap {
	jenkinsConfig := fmt.Sprintf(JenkinsConfig, image.Resolve(v3.ToolsSystemImages.PipelineSystemImages.JenkinsJnlp))

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.PipelineNamespace,
			Name:      "jenkins",
		},
		Data: map[string]string{
			"config.xml":      jenkinsConfig,
			"apply_config.sh": JenkinsApplyConfig,
			"plugins.txt":     JenkinsPlugins,
			"init.groovy":     InitGroovy,
		},
	}
}

func getServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.PipelineNamespace,
			Name:      "jenkins",
		},
	}
}

func getRoleBindings() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "jenkins-role-binding",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Namespace: utils.PipelineNamespace,
			Name:      "jenkins",
		}},
	}
}
func getJenkinsDeployment() *appsv1beta2.Deployment {
	replicas := int32(1)
	return &appsv1beta2.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.PipelineNamespace,
			Name:      "jenkins",
		},
		Spec: appsv1beta2.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "jenkins"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "jenkins"},
					Name:   "jenkins",
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "jenkins",
					InitContainers: []corev1.Container{
						{
							Name:            "jenkins-config",
							Image:           image.Resolve(v3.ToolsSystemImages.PipelineSystemImages.Jenkins),
							ImagePullPolicy: corev1.PullAlways,
							Command:         []string{"sh", "/var/jenkins_config/apply_config.sh"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "jenkins-home",
									MountPath: "/var/jenkins_home",
								},
								{
									Name:      "jenkins-config",
									MountPath: "/var/jenkins_config",
								},
								{
									Name:      "plugin-dir",
									MountPath: "/usr/share/jenkins/ref/plugins/",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "jenkins",
							Image:           image.Resolve(v3.ToolsSystemImages.PipelineSystemImages.Jenkins),
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
								{
									Name: "ADMIN_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "jenkins",
											},
											Key: "jenkins-admin-password",
										}},
								}, {
									Name: "ADMIN_USER",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "jenkins",
											},
											Key: "jenkins-admin-user",
										}},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
								},
								{
									Name:          "agent",
									ContainerPort: 50000,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "jenkins-home",
									MountPath: "/var/jenkins_home",
								},
								{
									Name:      "jenkins-config",
									MountPath: "/var/jenkins_config",
								},
								{
									Name:      "plugin-dir",
									MountPath: "/usr/share/jenkins/ref/plugins/",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "jenkins-home",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "jenkins-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "jenkins",
									},
								},
							},
						},
						{
							Name: "plugin-dir",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
}

const JenkinsConfig = `<?xml version='1.0' encoding='UTF-8'?>
<hudson>
  <disabledAdministrativeMonitors/>
  <version>lts</version>
  <numExecutors>0</numExecutors>
  <mode>NORMAL</mode>
  <authorizationStrategy class="hudson.security.FullControlOnceLoggedInAuthorizationStrategy">
    <denyAnonymousReadAccess>true</denyAnonymousReadAccess>
  </authorizationStrategy>
  <securityRealm class="hudson.security.LegacySecurityRealm"/>
  <disableRememberMe>false</disableRememberMe>
  <projectNamingStrategy class="jenkins.model.ProjectNamingStrategy$DefaultProjectNamingStrategy"/>
  <workspaceDir>${JENKINS_HOME}/workspace/${ITEM_FULLNAME}</workspaceDir>
  <buildsDir>${ITEM_ROOTDIR}/builds</buildsDir>
  <markupFormatter class="hudson.markup.EscapedMarkupFormatter"/>
  <jdks/>
  <viewsTabBar class="hudson.views.DefaultViewsTabBar"/>
  <myViewsTabBar class="hudson.views.DefaultMyViewsTabBar"/>
  <clouds>
    <org.csanchez.jenkins.plugins.kubernetes.KubernetesCloud plugin="kubernetes@1.1">
      <name>kubernetes</name>
      <templates>
        <org.csanchez.jenkins.plugins.kubernetes.PodTemplate>
          <inheritFrom></inheritFrom>
          <name>default</name>
          <instanceCap>2147483647</instanceCap>
          <idleMinutes>0</idleMinutes>
          <label>jenkins-slave</label>
          <nodeSelector></nodeSelector>
            <nodeUsageMode>NORMAL</nodeUsageMode>
          <volumes>
          </volumes>
          <containers>
            <org.csanchez.jenkins.plugins.kubernetes.ContainerTemplate>
              <name>jnlp</name>
              <image>%s</image>
              <privileged>false</privileged>
              <alwaysPullImage>false</alwaysPullImage>
              <workingDir>/home/jenkins</workingDir>
              <command></command>
              <args>${computer.jnlpmac} ${computer.name}</args>
              <ttyEnabled>false</ttyEnabled>
              <resourceRequestCpu>200m</resourceRequestCpu>
              <resourceRequestMemory>256Mi</resourceRequestMemory>
              <resourceLimitCpu>200m</resourceLimitCpu>
              <resourceLimitMemory>256Mi</resourceLimitMemory>
              <envVars>
                <org.csanchez.jenkins.plugins.kubernetes.ContainerEnvVar>
                  <key>JENKINS_URL</key>
                  <value>http://jenkins:8080</value>
                </org.csanchez.jenkins.plugins.kubernetes.ContainerEnvVar>
              </envVars>
            </org.csanchez.jenkins.plugins.kubernetes.ContainerTemplate>
          </containers>
          <envVars/>
          <annotations/>
          <imagePullSecrets/>
          <nodeProperties/>
        </org.csanchez.jenkins.plugins.kubernetes.PodTemplate></templates>
      <serverUrl>https://kubernetes.default</serverUrl>
      <skipTlsVerify>false</skipTlsVerify>
      <namespace>cattle-pipeline</namespace>
      <jenkinsUrl>http://jenkins:8080</jenkinsUrl>
      <jenkinsTunnel>jenkins:50000</jenkinsTunnel>
      <containerCap>10</containerCap>
      <retentionTimeout>5</retentionTimeout>
      <connectTimeout>0</connectTimeout>
      <readTimeout>0</readTimeout>
    </org.csanchez.jenkins.plugins.kubernetes.KubernetesCloud>
  </clouds>
  <quietPeriod>5</quietPeriod>
  <scmCheckoutRetryCount>0</scmCheckoutRetryCount>
  <views>
    <hudson.model.AllView>
      <owner class="hudson" reference="../../.."/>
      <name>All</name>
      <filterExecutors>false</filterExecutors>
      <filterQueue>false</filterQueue>
      <properties class="hudson.model.View$PropertyList"/>
    </hudson.model.AllView>
  </views>
  <primaryView>All</primaryView>
  <slaveAgentPort>50000</slaveAgentPort>
  <crumbIssuer class="hudson.security.csrf.DefaultCrumbIssuer">
    <excludeClientIPFromCrumb>false</excludeClientIPFromCrumb>
  </crumbIssuer>
  <label></label>
  <nodeProperties/>
  <globalNodeProperties/>
  <noUsageStatistics>true</noUsageStatistics>
</hudson>`

const InitGroovy = `
import jenkins.model.*
import hudson.security.*

def instance = Jenkins.getInstance()
def hudsonRealm = new HudsonPrivateSecurityRealm(false)
def env = System.getenv()
def user = env['ADMIN_USER']
def passowrd = env['ADMIN_PASSWORD']
hudsonRealm.createAccount(user,passowrd)
instance.setSecurityRealm(hudsonRealm)
def strategy = new hudson.security.FullControlOnceLoggedInAuthorizationStrategy()
strategy.setAllowAnonymousRead(false)
instance.setAuthorizationStrategy(strategy)
instance.save()
`
const JenkinsApplyConfig = `
mkdir -p /var/jenkins_home/init.groovy.d
cp /var/jenkins_config/init.groovy /var/jenkins_home/init.groovy.d
cp -n /var/jenkins_config/config.xml /var/jenkins_home;
cp /var/jenkins_config/plugins.txt /var/jenkins_home;
rm -rf /usr/share/jenkins/ref/plugins/*.lock
/usr/local/bin/install-plugins.sh ` + "`echo $(cat /var/jenkins_home/plugins.txt)`;"

const JenkinsPlugins = `
kubernetes:1.1.4
timestamper:1.8.9
workflow-aggregator:2.5
workflow-job:2.17
credentials-binding:1.13
git:3.6.4`
