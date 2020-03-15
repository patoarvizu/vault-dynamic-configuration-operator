package e2e

import (
	"testing"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
)

func TestSingleNamespaceServiceAccountAnnotation(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	setup(t, ctx)
	createServiceAccount("operator-test", "default", map[string]string{}, ctx)
	testVaultRole("operator-test", []string{"default"}, t)
}

func TestSingleNamespaceServiceAccountDBAnnotation(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	setup(t, ctx)
	createServiceAccount("operator-test-db", "default", map[string]string{"vault.patoarvizu.dev/db-dynamic-creds": "mysql"}, ctx)
	testVaultRole("operator-test-db", []string{"default"}, t)
	testVaultDBRole("operator-test-db", t)
}

func TestSingleNamespaceMultipleAccounts(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	setup(t, ctx)
	createServiceAccount("operator-test-multi-ns", "test-vdc1", map[string]string{}, ctx)
	createServiceAccount("operator-test-multi-ns", "test-vdc2", map[string]string{}, ctx)
	testVaultRole("operator-test-multi-ns", []string{"test-vdc1", "test-vdc2"}, t)
}

func TestAllNamespacesServiceAccountAnnotation(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	setup(t, ctx)
	createServiceAccount("operator-test-all", "default", map[string]string{}, ctx)
	testVaultRole("operator-test-all", []string{"*"}, t)
}
