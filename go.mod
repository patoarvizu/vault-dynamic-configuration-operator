module github.com/patoarvizu/vault-dynamic-configuration-operator

go 1.16

require (
	github.com/banzaicloud/bank-vaults v1.14.3-0.20211011063455-e2138a966538
	github.com/go-logr/logr v0.4.0
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.10.5
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	sigs.k8s.io/controller-runtime v0.9.0-beta.5
)

replace (
	github.com/go-logr/zapr => github.com/go-logr/zapr v0.4.0
	github.com/onsi/ginkgo => github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega => github.com/onsi/gomega v1.10.1
	google.golang.org/grpc => google.golang.org/grpc v1.29.1
	k8s.io/api => k8s.io/api v0.19.3
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.3
	k8s.io/client-go => k8s.io/client-go v0.19.3
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.6.2
)
