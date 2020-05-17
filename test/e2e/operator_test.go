package e2e

import (
	"context"
	"testing"
	"time"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	dynclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestMonitoringObjectsCreated(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	setup(t, ctx)
	metricsService := &v1.Service{}
	err := wait.Poll(time.Second*2, time.Second*60, func() (done bool, err error) {
		err = framework.Global.Client.Get(context.TODO(), dynclient.ObjectKey{Namespace: "vault", Name: "vault-dynamic-configuration-operator-metrics"}, metricsService)
		if err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		t.Fatal("Could not get metrics Service")
	}
	httpMetricsPortFound := false
	crMetricsPortFound := false
	for _, p := range metricsService.Spec.Ports {
		if p.Name == "http-metrics" && p.Port == 8383 {
			httpMetricsPortFound = true
			continue
		}
		if p.Name == "cr-metrics" && p.Port == 8686 {
			crMetricsPortFound = true
			continue
		}
	}
	if !httpMetricsPortFound {
		t.Fatal("Service vault-dynamic-configuration-operator-metrics doesn't have http-metrics port 8383")
	}
	if !crMetricsPortFound {
		t.Fatal("Service vault-dynamic-configuration-operator-metrics doesn't have cr-metrics port 8686")
	}

	framework.Global.Scheme.AddKnownTypes(monitoringv1.SchemeGroupVersion, &monitoringv1.ServiceMonitor{})
	serviceMonitor := &monitoringv1.ServiceMonitor{}
	err = wait.Poll(time.Second*2, time.Second*60, func() (done bool, err error) {
		err = framework.Global.Client.Client.Get(context.TODO(), dynclient.ObjectKey{Namespace: "vault", Name: "vault-dynamic-configuration-operator-metrics"}, serviceMonitor)
		if err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		t.Fatal("Could not find metrics ServiceMonitor")
	}
	httpMetricsEndpointFound := false
	crMetricsEndpointFound := false
	for _, e := range serviceMonitor.Spec.Endpoints {
		if e.Port == "http-metrics" {
			httpMetricsEndpointFound = true
			continue
		}
		if e.Port == "cr-metrics" {
			crMetricsEndpointFound = true
			continue
		}
	}
	if !httpMetricsEndpointFound {
		t.Error("ServiceMonitor vault-dynamic-configuration-operator-metrics doesn't have endpoint http-metrics")
	}
	if !crMetricsEndpointFound {
		t.Error("ServiceMonitor vault-dynamic-configuration-operator-metrics doesn't have endpoint cr-metrics")
	}
}

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
