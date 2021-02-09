package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	bankvaultsv1alpha1 "github.com/banzaicloud/bank-vaults/operator/pkg/apis/vault/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/patoarvizu/vault-dynamic-configuration-operator/controllers"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster: newTrue(),
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = bankvaultsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	close(done)
}, 60)

var _ = Describe("Single namespace", func() {
	var (
		serviceAccount1 *apiv1.ServiceAccount
		serviceAccount2 *apiv1.ServiceAccount
		err             error
	)
	Context("When service acount has the annotation", func() {
		It("Should create the corresponding Vault role", func() {
			serviceAccount1, err = createServiceAccount("operator-test", "default", map[string]string{})
			Expect(err).ToNot(HaveOccurred())
			err = testVaultRole("operator-test", []string{"default"})
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(context.TODO(), serviceAccount1)
			Expect(err).ToNot(HaveOccurred())
		})
	})
	Context("When service account has the DB role annotation", func() {
		It("Should create the corresponding Vault roles", func() {
			serviceAccount1, err = createServiceAccount("operator-test-db", "default", map[string]string{"vault.patoarvizu.dev/db-dynamic-creds": "mysql"})
			Expect(err).ToNot(HaveOccurred())
			err = testVaultRole("operator-test-db", []string{"default"})
			Expect(err).ToNot(HaveOccurred())
			err = testVaultDBRole("operator-test-db")
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(context.TODO(), serviceAccount1)
			Expect(err).ToNot(HaveOccurred())
		})
	})
	Context("When multiple service accounts are created", func() {
		It("Should create a Vault role for each service account", func() {
			serviceAccount1, err = createServiceAccount("operator-test-multi-ns", "test-vdc1", map[string]string{})
			Expect(err).ToNot(HaveOccurred())
			serviceAccount2, err = createServiceAccount("operator-test-multi-ns", "test-vdc2", map[string]string{})
			Expect(err).ToNot(HaveOccurred())
			err = testVaultRole("operator-test-multi-ns", []string{"test-vdc1", "test-vdc2"})
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(context.TODO(), serviceAccount1)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(context.TODO(), serviceAccount2)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("All namespaces", func() {
	var (
		serviceAccount *apiv1.ServiceAccount
		err            error
	)
	Context("When a service account has the annotation", func() {
		It("Should create a Vault role that is bound to all namespaces", func() {
			serviceAccount, err = createServiceAccount("operator-test-all", "default", map[string]string{})
			Expect(err).ToNot(HaveOccurred())
			err = testVaultRole("operator-test-all", []string{"*"})
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(context.TODO(), serviceAccount)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("Any namespace", func() {
	Context("When annotating a service account called 'default'", func() {
		It("Should NOT create a Vault role or policy wit that name", func() {
			serviceAccount, err := createServiceAccount("default", "default", map[string]string{})
			Expect(err).ToNot(HaveOccurred())
			err = testVaultRole("default", []string{"*"})
			Expect(err).To(HaveOccurred())
			err = k8sClient.Delete(context.TODO(), serviceAccount)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func newTrue() *bool {
	b := true
	return &b
}

func createServiceAccount(name string, namespace string, extraAnnotations map[string]string) (serviceAccount *apiv1.ServiceAccount, err error) {
	annotations := map[string]string{"vault.patoarvizu.dev/auto-configure": "true"}
	for k, v := range extraAnnotations {
		annotations[k] = v
	}
	var operatorTestServiceAccount = &apiv1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
	}
	e := k8sClient.Create(context.TODO(), operatorTestServiceAccount)
	return operatorTestServiceAccount, e
}

func namespaceIsInAllowedList(namespace string, allowedNamespaces interface{}) bool {
	for _, ns := range allowedNamespaces.([]interface{}) {
		if ns.(string) == namespace {
			return true
		}
	}
	return false
}

func testVaultRole(name string, namespaces []string) error {
	vaultCR := &bankvaultsv1alpha1.Vault{}
	bvConfig := controllers.BankVaultsConfig{}
	err := wait.Poll(time.Second*2, time.Second*20, func() (done bool, err error) {
		k8sClient.Get(context.TODO(), types.NamespacedName{Name: "vault", Namespace: "vault"}, vaultCR)
		jsonData, wErr := json.Marshal(vaultCR.Spec.ExternalConfig)
		if wErr != nil {
			return false, nil
		}
		wErr = json.Unmarshal(jsonData, &bvConfig)
		if wErr != nil {
			return false, nil
		}
		role, wErr := bvConfig.GetRole(name)
		if wErr != nil {
			return false, nil
		}
		if role.BoundServiceAccountNames != name || role.TokenTtl != "5m" {
			return true, errors.New(fmt.Sprintf("Test role '%s' is not configured correctly", name))
		}
		if len(role.BoundServiceAccountNamespaces.([]interface{})) < len(namespaces) {
			return false, nil
		}
		for _, ns := range namespaces {
			if !namespaceIsInAllowedList(ns, role.BoundServiceAccountNamespaces) {
				return true, errors.New(fmt.Sprintf("Namespace '%s' is not in list of role bound namespaces", ns))
			}
		}
		if role.TokenPolicies[0] != name {
			return true, errors.New(fmt.Sprintf("Test role '%s' policies are not configured correctly", name))
		}
		policy, wErr := bvConfig.GetPolicy(name)
		if wErr != nil {
			return false, nil
		}
		if policy.Rules != fmt.Sprintf("path \"secret/%s\" {\n  capabilities = [\"read\"]\n}\n", name) {
			return true, errors.New(fmt.Sprintf("Test role '%s' policy rules are not configured correctly", name))
		}
		return true, nil
	})
	return err
}

func testVaultDBRole(name string) error {
	vaultCR := &bankvaultsv1alpha1.Vault{}
	bvConfig := controllers.BankVaultsConfig{}
	err := wait.Poll(time.Second*2, time.Second*20, func() (done bool, err error) {
		k8sClient.Get(context.TODO(), types.NamespacedName{Name: "vault", Namespace: "vault"}, vaultCR)
		jsonData, wErr := json.Marshal(vaultCR.Spec.ExternalConfig)
		if wErr != nil {
			return false, nil
		}
		wErr = json.Unmarshal(jsonData, &bvConfig)
		if err != nil {
			return false, nil
		}
		role, wErr := bvConfig.GetDBRole(name)
		if wErr != nil {
			return false, nil
		}
		if role.DbName != "mysql" || role.DefaultTtl != "1h" || role.MaxTtl != "24h" {
			return true, errors.New(fmt.Sprintf("Dynamic DB credentials for role '%s' aren't configured correctly", name))
		}
		dbSecret, wErr := bvConfig.GetDBSecret()
		if wErr != nil {
			fmt.Print("No dbSecretsConfiguration")
			return false, nil
		}
		dbConfig, wErr := dbSecret.Configuration.GetDBConfig("mysql")
		if wErr != nil {
			fmt.Print("No dbConfig")
			return false, nil
		}
		if dbConfig.AllowedRoles[0] != name {
			return true, errors.New(fmt.Sprintf("Role '%s' configured for dynamic DB credentials is missing from allowed_roles", name))
		}
		return true, nil
	})
	return err
}
