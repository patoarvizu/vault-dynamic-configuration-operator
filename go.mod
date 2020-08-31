module github.com/patoarvizu/vault-dynamic-configuration-operator

go 1.13

require (
	github.com/Masterminds/semver/v3 v3.1.0
	github.com/banzaicloud/bank-vaults v0.0.0-20200825124647-f70bdd822e23
	github.com/coreos/prometheus-operator v0.41.1
	github.com/go-logr/logr v0.1.0
	github.com/google/martian v2.1.1-0.20190517191504-25dcb96d9e51+incompatible
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/spf13/cast v1.3.1
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/utils v0.0.0-20200729134348-d5654de09c73
	sigs.k8s.io/controller-runtime v0.6.2
)

replace k8s.io/client-go => k8s.io/client-go v0.18.6
