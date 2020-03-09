package e2e

import (
	"context"
	"testing"
	"time"

	bankvaultsv1alpha1 "github.com/banzaicloud/bank-vaults/operator/pkg/apis/vault/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/test"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
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

func createServiceAccount(name string, ctx *test.TestCtx) error {
	var opertatorTestServiceAccount = &apiv1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Annotations: map[string]string{
				"vault.patoarvizu.dev/auto-configure": "true",
			},
		},
	}
	return framework.Global.Client.Create(context.TODO(), opertatorTestServiceAccount, &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second * 60, RetryInterval: time.Second * 1})
}

func testVaultRole(name string, namespace string, t *testing.T) {
	vaultCR := &bankvaultsv1alpha1.Vault{}
	wait.Poll(time.Second*2, time.Second*60, func() (done bool, err error) {
		framework.Global.Client.Get(context.TODO(), types.NamespacedName{Name: "vault", Namespace: "vault"}, vaultCR)
		auth := vaultCR.Spec.ExternalConfig["auth"]
		auth0 := auth.([]interface{})[0]
		roles := auth0.(map[string]interface{})["roles"]
		if roles == nil {
			return false, nil
		}
		var role map[string]interface{}
		for _, r := range roles.([]interface{}) {
			rn := r.(map[string]interface{})
			if rn["name"] != name {
				continue
			}
			role = rn
		}
		if role["name"] != name {
			return false, nil
		}
		if role["bound_service_account_names"] != name || role["bound_service_account_namespaces"] != namespace {
			t.Errorf("Test role '%s' is not congigured correctly", name)
		}
		return true, nil
	})
}
