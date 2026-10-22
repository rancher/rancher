package autoscaler

var k8sVersionToAutoscalerChartVersions = map[int]*k8sToAutoscalerVersion{
	36: {
		imageTag:     "1.36.1-3.1",
		chartVersion: "9.59.0",
	},
	32: {
		imageTag:     "1.32.3-1.5",
		chartVersion: "9.50.1",
	},
	31: {
		imageTag:     "1.31.4-2.0",
		chartVersion: "9.48.0",
	},
}
