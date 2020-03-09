package e2e

import (
	"testing"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
)

func TestSingleNamespaceServiceAccountAnnotation(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	setup(t, ctx)
	createServiceAccount("operator-test", ctx)
	testVaultRole("operator-test", "default", t)
}

func TestAllNamespacesServiceAccountAnnotation(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	setup(t, ctx)
	createServiceAccount("operator-test-all", ctx)
	testVaultRole("operator-test-all", "*", t)
}
