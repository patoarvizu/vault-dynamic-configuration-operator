module github.com/patoarvizu/vault-dynamic-configuration-operator

go 1.13

require (
	github.com/banzaicloud/bank-vaults v1.7.1-0.20210215124259-db2bdd2dc82d
	github.com/go-logr/logr v0.2.1
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.1
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v11.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.2
)

replace (
	google.golang.org/grpc => google.golang.org/grpc v1.29.1
	k8s.io/api => k8s.io/api v0.19.3
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.3
	k8s.io/client-go => k8s.io/client-go v0.19.3
)
