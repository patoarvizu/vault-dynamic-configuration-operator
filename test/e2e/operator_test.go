package e2e

import (
	"testing"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
)

func TestSingleNamespaceServiceAccountAnnotation(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	setup(t, ctx)
	createServiceAccount("operator-test", map[string]string{}, ctx)
	testVaultRole("operator-test", "default", t)
}

func TestSingleNamespaceServiceAccountDBAnnotation(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	setup(t, ctx)
	createServiceAccount("operator-test-db", map[string]string{"vault.patoarvizu.dev/db-dynamic-creds": "mysql"}, ctx)
	testVaultRole("operator-test-db", "default", t)
	testVaultDBRole("operator-test-db", t)
}

func TestAllNamespacesServiceAccountAnnotation(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	setup(t, ctx)
	createServiceAccount("operator-test-all", map[string]string{}, ctx)
	testVaultRole("operator-test-all", "*", t)
}
