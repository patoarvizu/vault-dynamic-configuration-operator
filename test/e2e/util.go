package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	bankvaultsv1alpha1 "github.com/banzaicloud/bank-vaults/operator/pkg/apis/vault/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/test"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"github.com/patoarvizu/vault-dynamic-configuration-operator/pkg/controller/vdc"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

func setup(t *testing.T, ctx *test.TestCtx) {
	vaultList := &bankvaultsv1alpha1.VaultList{}
	err := framework.AddToFrameworkScheme(bankvaultsv1alpha1.AddToScheme, vaultList)
	if err != nil {
		t.Fatalf("Failed to add Vault CRD schema to framework: %v", err)
	}
	err = e2eutil.WaitForOperatorDeployment(t, framework.Global.KubeClient, "vault", "vault-dynamic-configuration-operator", 1, time.Second*5, time.Second*60)
	if err != nil {
		t.Fatal(err)
	}
}

func createServiceAccount(name string, extraAnnotations map[string]string, ctx *test.TestCtx) error {
	annotations := map[string]string{"vault.patoarvizu.dev/auto-configure": "true"}
	for k, v := range extraAnnotations {
		annotations[k] = v
	}
	var opertatorTestServiceAccount = &apiv1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "default",
			Annotations: annotations,
		},
	}
	return framework.Global.Client.Create(context.TODO(), opertatorTestServiceAccount, &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second * 60, RetryInterval: time.Second * 1})
}

func testVaultRole(name string, namespace string, t *testing.T) {
	vaultCR := &bankvaultsv1alpha1.Vault{}
	bvConfig := vdc.BankVaultsConfig{}
	err := wait.Poll(time.Second*2, time.Second*20, func() (done bool, err error) {
		framework.Global.Client.Get(context.TODO(), types.NamespacedName{Name: "vault", Namespace: "vault"}, vaultCR)
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
		if role.BoundServiceAccountNames != name || role.BoundServiceAccountNamespaces[0] != namespace || role.TokenTtl != "5m" {
			t.Errorf("Test role '%s' is not configured correctly", name)
		}
		if role.TokenPolicies[0] != name {
			t.Errorf("Test role '%s' policies are not configured correctly", name)
		}
		policy, wErr := bvConfig.GetPolicy(name)
		if wErr != nil {
			return false, nil
		}
		if policy.Rules != fmt.Sprintf("path \"secret/%s\" {\n  capabilities = [\"read\"]\n}\n", name) {
			t.Errorf("Test role '%s' policy rules are not configured correctly", name)
		}
		return true, nil
	})
	if err != nil {
		t.Errorf("Error: %v", err)
	}
}

func testVaultDBRole(name string, t *testing.T) {
	vaultCR := &bankvaultsv1alpha1.Vault{}
	bvConfig := vdc.BankVaultsConfig{}
	err := wait.Poll(time.Second*2, time.Second*20, func() (done bool, err error) {
		framework.Global.Client.Get(context.TODO(), types.NamespacedName{Name: "vault", Namespace: "vault"}, vaultCR)
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
			t.Errorf("Dynamic DB credentials for role '%s' aren't configured correctly", name)
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
			t.Errorf("Role '%s' configured for dynamic DB credentials is missing from allowed_roles", name)
		}
		return true, nil
	})
	if err != nil {
		t.Errorf("Error: %v", err)
	}
}
