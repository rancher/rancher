package util

var FeatureAppNS = []string{
	"ingress-nginx",              // This is for Ingress, not feature app
	"kube-system",                // Harvester, vSphere CPI, vSphere CSI
	"cattle-system",              // AKS/GKE/EKS Operator, Webhook, System Upgrade Controller
	"cattle-epinio-system",       // Epinio
	"cattle-fleet-system",        // Fleet
	"longhorn-system",            // Longhorn
	"cattle-neuvector-system",    // Neuvector
	"cattle-monitoring-system",   // Monitoring and Sub-charts
	"rancher-alerting-drivers",   // Alert Driver
	"cis-operator-system",        // CIS Benchmark
	"cattle-csp-adapter-system",  // CSP Adapter
	"cattle-externalip-system",   // External IP Webhook
	"cattle-gatekeeper-system",   // Gatekeeper
	"istio-system",               // Istio and Sub-charts
	"cattle-istio-system",        // Kiali
	"cattle-logging-system",      // Logging
	"cattle-windows-gmsa-system", // Windows GMSA
	"cattle-sriov-system",        // Sriov
	"cattle-ui-plugin-system",    // UI Plugin System
}
