package windows

import (
	"testing"

	util "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func Test_canDeploy(t *testing.T) {
	type testcase struct {
		name      string
		target    *util.Workload
		canDeploy bool
	}
	cases := []testcase{
		testcase{
			name: "test workload with linux node selector",
			target: &util.Workload{
				TemplateSpec: &v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						NodeSelector: map[string]string{"beta.kubernetes.io/os": "linux"},
					},
				},
			},
			canDeploy: true,
		},
		testcase{
			name: "test workload with linux node affinity",
			target: &util.Workload{
				TemplateSpec: &v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Affinity: &v1.Affinity{
							NodeAffinity: &v1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
									NodeSelectorTerms: []v1.NodeSelectorTerm{
										v1.NodeSelectorTerm{
											MatchExpressions: []v1.NodeSelectorRequirement{
												v1.NodeSelectorRequirement{
													Key:      "beta.kubernetes.io/os",
													Operator: v1.NodeSelectorOpIn,
													Values:   []string{"linux"},
												},
												v1.NodeSelectorRequirement{
													Key:      "not-related",
													Operator: v1.NodeSelectorOpIn,
													Values:   []string{"not-related"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			canDeploy: true,
		},
		testcase{
			name: "test workload with non-related node affinity",
			target: &util.Workload{
				TemplateSpec: &v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Affinity: &v1.Affinity{
							NodeAffinity: &v1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
									NodeSelectorTerms: []v1.NodeSelectorTerm{
										v1.NodeSelectorTerm{
											MatchExpressions: []v1.NodeSelectorRequirement{
												v1.NodeSelectorRequirement{
													Key:      "not-related",
													Operator: v1.NodeSelectorOpIn,
													Values:   []string{"not-related"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			canDeploy: false,
		},
		testcase{
			name: "test workload without node selector and node affinity",
			target: &util.Workload{
				TemplateSpec: &v1.PodTemplateSpec{
					Spec: v1.PodSpec{},
				},
			},
			canDeploy: false,
		},
	}

	for _, c := range cases {
		assert.Equalf(t, canDeployedIntoLinuxNode(c.target.TemplateSpec.Spec), c.canDeploy, "failed to tell which workload wants to be deployed into linux, test case: %s", c.name)
	}
}

type workloadRefenece struct {
	workload *util.Workload
	deploy   *v1beta2.Deployment
	rc       *v1.ReplicationController
	rs       *v1beta2.ReplicaSet
	ss       *v1beta2.StatefulSet
	ds       *v1beta2.DaemonSet
	job      *batchv1.Job
	cj       *batchv1beta1.CronJob
}

type workloadTestCase struct {
	name         string
	reference    *workloadRefenece
	shouldUpdate bool
}

func Test_workloadController(t *testing.T) {
	cases := []workloadTestCase{
		workloadTestCase{
			name: "test deployment without node selector",
			reference: &workloadRefenece{
				workload: &util.Workload{
					Name:         "deploy1",
					Namespace:    "default",
					Key:          "deployment:default:deploy1",
					Kind:         util.DeploymentType,
					TemplateSpec: &v1.PodTemplateSpec{},
				},
				deploy: &v1beta2.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "deploy1",
						Namespace: "default",
					},
					Spec: v1beta2.DeploymentSpec{
						Template: v1.PodTemplateSpec{},
					},
				},
			},
			shouldUpdate: false,
		},
		workloadTestCase{
			name: "test statefulset with linux node selector gets pod toleration updated",
			reference: &workloadRefenece{
				workload: &util.Workload{
					Name:      "ss1",
					Namespace: "default",
					Key:       "statefulset:default:ss1",
					Kind:      util.StatefulSetType,
					TemplateSpec: &v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							NodeSelector: map[string]string{"beta.kubernetes.io/os": "linux"},
						},
					},
				},
				ss: &v1beta2.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ss1",
						Namespace: "default",
					},
					Spec: v1beta2.StatefulSetSpec{
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								NodeSelector: map[string]string{"beta.kubernetes.io/os": "linux"},
							},
						},
					},
				},
			},
			shouldUpdate: true,
		},
		workloadTestCase{
			name: "test replicationcontroller with linux node affinity gets pod toleration updated",
			reference: &workloadRefenece{
				workload: &util.Workload{
					Name:      "rc1",
					Namespace: "default",
					Key:       "replicationcontroller:default:rc1",
					Kind:      util.ReplicationControllerType,
					TemplateSpec: &v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Affinity: &v1.Affinity{
								NodeAffinity: &v1.NodeAffinity{
									RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
										NodeSelectorTerms: []v1.NodeSelectorTerm{
											v1.NodeSelectorTerm{
												MatchExpressions: []v1.NodeSelectorRequirement{
													v1.NodeSelectorRequirement{
														Key:      "beta.kubernetes.io/os",
														Operator: v1.NodeSelectorOpIn,
														Values:   []string{"linux"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				rc: &v1.ReplicationController{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rc1",
						Namespace: "default",
					},
					Spec: v1.ReplicationControllerSpec{
						Template: &v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								Affinity: &v1.Affinity{
									NodeAffinity: &v1.NodeAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
											NodeSelectorTerms: []v1.NodeSelectorTerm{
												v1.NodeSelectorTerm{
													MatchExpressions: []v1.NodeSelectorRequirement{
														v1.NodeSelectorRequirement{
															Key:      "beta.kubernetes.io/os",
															Operator: v1.NodeSelectorOpIn,
															Values:   []string{"linux"},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			shouldUpdate: true,
		},
		workloadTestCase{
			name: "test replicaset with not-in-windows node affinity gets pod toleration updated",
			reference: &workloadRefenece{
				workload: &util.Workload{
					Name:      "rs1",
					Namespace: "default",
					Key:       "replicaset:default:rs1",
					Kind:      util.ReplicaSetType,
					TemplateSpec: &v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Affinity: &v1.Affinity{
								NodeAffinity: &v1.NodeAffinity{
									RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
										NodeSelectorTerms: []v1.NodeSelectorTerm{
											v1.NodeSelectorTerm{
												MatchExpressions: []v1.NodeSelectorRequirement{
													v1.NodeSelectorRequirement{
														Key:      "beta.kubernetes.io/os",
														Operator: v1.NodeSelectorOpNotIn,
														Values:   []string{"windows"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				rs: &v1beta2.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rs1",
						Namespace: "default",
					},
					Spec: v1beta2.ReplicaSetSpec{
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								Affinity: &v1.Affinity{
									NodeAffinity: &v1.NodeAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
											NodeSelectorTerms: []v1.NodeSelectorTerm{
												v1.NodeSelectorTerm{
													MatchExpressions: []v1.NodeSelectorRequirement{
														v1.NodeSelectorRequirement{
															Key:      "beta.kubernetes.io/os",
															Operator: v1.NodeSelectorOpNotIn,
															Values:   []string{"windows"},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			shouldUpdate: true,
		},
	}
	workloads := getWorkloadsFromCases(cases)
	wc := newFakeCommonController(cases, workloads)
	controller := &WorkloadTolerationHandler{
		workloadController: wc,
	}
	for _, c := range cases {
		err := controller.sync(c.reference.workload.Key, c.reference.workload)
		assert.Nilf(t, err, "failed to sync workload %s in case %s", c.reference.workload.Key, c.name)
	}
}

func getItemsFromWorkload(wrs []workloadRefenece) []runtime.Object {
	var rtn []runtime.Object
	for _, workload := range wrs {
		var obj runtime.Object
		switch {
		case workload.deploy != nil:
			obj = workload.deploy
		case workload.rc != nil:
			obj = workload.rc
		case workload.rs != nil:
			obj = workload.rs
		case workload.ss != nil:
			obj = workload.ss
		case workload.ds != nil:
			obj = workload.ds
		case workload.job != nil:
			obj = workload.job
		case workload.cj != nil:
			obj = workload.cj
		default:
			continue
		}
		rtn = append(rtn, obj)
	}
	return rtn
}

func getWorkloadsFromCases(cases []workloadTestCase) []workloadRefenece {
	var rtn []workloadRefenece
	for _, c := range cases {
		rtn = append(rtn, *c.reference)
	}
	return rtn
}
