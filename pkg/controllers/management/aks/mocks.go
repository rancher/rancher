//go:generate mockcompose -n mockAksOperatorController -c aksOperatorController -real setInitialUpstreamSpec -real generateAndSetServiceAccount -real generateSATokenWithPublicAPI -mock getRestConfig
//go:generate mockcompose -n mockClusterOperator -p github.com/rancher/rancher/pkg/controllers/management/clusteroperator -mock GenerateSAToken
package aks
