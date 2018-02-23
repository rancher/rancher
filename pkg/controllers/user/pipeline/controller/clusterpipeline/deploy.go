package clusterpipeline

import (
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/engine/jenkins"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	JenkinsImage = "jenkins/jenkins:lts"
)

func getSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.PipelineNamespace,
			Name:      "jenkins",
		},
		Data: map[string][]byte{
			"jenkins-admin-password": []byte(jenkins.JenkinsDefaultToken),
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
			Type: corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "jenkins",
			},
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 8080,
				},
			},
		},
	}
}

func getJenkinsAgentService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.PipelineNamespace,
			Name:      "jenkins-agent",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "jenkins",
			},
			Ports: []corev1.ServicePort{
				{
					Name: "agent",
					Port: 50000,
				},
			},
		},
	}
}

func getConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.PipelineNamespace,
			Name:      "jenkins",
		},
		Data: map[string]string{
			"config.xml":      JenkinsConfig,
			"apply_config.sh": JenkinsApplyConfig,
			"plugins.txt":     JenkinsPlugins,
			"user-config.xml": JenkinsUserConfig,
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
							Image:           JenkinsImage,
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
							Image:           JenkinsImage,
							ImagePullPolicy: corev1.PullAlways,
							Args: []string{"--argumentsRealm.passwd.$(ADMIN_USER)=$(ADMIN_PASSWORD)",
								"--argumentsRealm.roles.$(ADMIN_USER)=admin"},
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

const JenkinsConfig = `
<?xml version='1.0' encoding='UTF-8'?>
<hudson>
  <disabledAdministrativeMonitors/>
  <version>lts</version>
  <numExecutors>0</numExecutors>
  <mode>NORMAL</mode>
  <useSecurity>false</useSecurity>
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
              <image>jenkins/jnlp-slave:3.10-1</image>
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
      <jenkinsTunnel>jenkins-agent:50000</jenkinsTunnel>
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

const JenkinsUserConfig = `
<?xml version='1.0' encoding='UTF-8'?>
<user>
  <fullName>admin</fullName>
  <description></description>
  <properties>
    <jenkins.security.ApiTokenProperty>
      <apiToken>{AQAAABAAAAAwiN1imgMxTBKVZ2f+imk9dhgpJb7NzJw6jaFz6YyP90wF2YLBWfsA5g+F6zeqXejv8y74WxKuWMcgzJ3bMhkTyw==}</apiToken>
    </jenkins.security.ApiTokenProperty>
    <com.cloudbees.plugins.credentials.UserCredentialsProvider_-UserCredentialsProperty plugin="credentials@2.1.14">
      <domainCredentialsMap class="hudson.util.CopyOnWriteMap$Hash"/>
    </com.cloudbees.plugins.credentials.UserCredentialsProvider_-UserCredentialsProperty>
    <hudson.tasks.Mailer_-UserProperty plugin="mailer@1.20">
      <emailAddress></emailAddress>
    </hudson.tasks.Mailer_-UserProperty>
    <hudson.plugins.emailext.watching.EmailExtWatchAction_-UserProperty plugin="email-ext@2.58">
      <triggers/>
    </hudson.plugins.emailext.watching.EmailExtWatchAction_-UserProperty>
    <jenkins.security.LastGrantedAuthoritiesProperty>
      <roles>
        <string>authenticated</string>
      </roles>
      <timestamp>1501052657373</timestamp>
    </jenkins.security.LastGrantedAuthoritiesProperty>
    <hudson.model.MyViewsProperty>
      <primaryViewName></primaryViewName>
      <views>
        <hudson.model.AllView>
          <owner class="hudson.model.MyViewsProperty" reference="../../.."/>
          <name>all</name>
          <filterExecutors>false</filterExecutors>
          <filterQueue>false</filterQueue>
          <properties class="hudson.model.View$PropertyList"/>
        </hudson.model.AllView>
      </views>
    </hudson.model.MyViewsProperty>
    <org.jenkinsci.plugins.displayurlapi.user.PreferredProviderUserProperty plugin="display-url-api@2.0">
      <providerId>default</providerId>
    </org.jenkinsci.plugins.displayurlapi.user.PreferredProviderUserProperty>
    <hudson.model.PaneStatusProperties>
      <collapsed/>
    </hudson.model.PaneStatusProperties>
    <hudson.security.HudsonPrivateSecurityRealm_-Details>
      <passwordHash>#jbcrypt:$2a$10$arbwzkBe0Uo6VrXUs//U3eL7k/tEtr2MayybwOP72Et7qTjeENqvK</passwordHash>
    </hudson.security.HudsonPrivateSecurityRealm_-Details>
    <org.jenkinsci.main.modules.cli.auth.ssh.UserPropertyImpl>
      <authorizedKeys></authorizedKeys>
    </org.jenkinsci.main.modules.cli.auth.ssh.UserPropertyImpl>
    <hudson.search.UserSearchProperty>
      <insensitiveSearch>true</insensitiveSearch>
    </hudson.search.UserSearchProperty>
  </properties>
</user>`
const JenkinsApplyConfig = `
mkdir -p /var/jenkins_home/users/admin && cp /var/jenkins_config/user-config.xml /var/jenkins_home/users/admin
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
