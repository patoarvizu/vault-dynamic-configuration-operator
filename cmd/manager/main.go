package main

import (
	"context"
	"encoding/gob"
	"flag"
	"fmt"
	"os"
	"runtime"

	"k8s.io/client-go/rest"

	bankvaultsv1alpha1 "github.com/banzaicloud/bank-vaults/operator/pkg/apis/vault/v1alpha1"
	"github.com/patoarvizu/vault-dynamic-configuration-operator/pkg/apis"
	"github.com/patoarvizu/vault-dynamic-configuration-operator/pkg/controller"
	"github.com/patoarvizu/vault-dynamic-configuration-operator/pkg/controller/vdc"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	kubemetrics "github.com/operator-framework/operator-sdk/pkg/kube-metrics"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	"github.com/operator-framework/operator-sdk/pkg/metrics"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

var (
	metricsHost               = "0.0.0.0"
	metricsPort         int32 = 8383
	operatorMetricsPort int32 = 8686
)
var log = logf.Log.WithName("cmd")

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion.Version))
}

func main() {
	pflag.CommandLine.AddFlagSet(zap.FlagSet())

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	pflag.StringVar(&vdc.TargetVaultName, "target-vault-name", "vault", "Name of Vault custom resource to target")
	pflag.StringVar(&vdc.AnnotationPrefix, "annotation-prefix", "vault.patoarvizu.dev", "Prefix of the annotations the operator should watch for in service accounts to configure roles and policies")
	pflag.StringVar(&vdc.AutoConfigureAnnotation, "auto-configure-annotation", "auto-configure", "Annotation the operator should watch for in service accounts")
	pflag.StringVar(&vdc.DynamicDBCredentialsAnnotation, "auto-configuredb-creds-annotation", "db-dynamic-creds", "Annotation the operator should watch for in service accounts to configure access to dynamic DB credentials")
	pflag.BoolVar(&vdc.BoundRolesToAllNamespaces, "bound-roles-to-all-namespaces", false, "Set 'bound_service_account_namespaces' to '*' instead of the service account's namespace")
	pflag.StringVar(&vdc.TokenTtl, "token-ttl", "5m", "Value to set roles' 'token_ttl' to")

	pflag.Parse()

	logf.SetLogger(zap.Logger())

	printVersion()

	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		log.Error(err, "Failed to get watch namespace")
		os.Exit(1)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	ctx := context.TODO()
	err = leader.Become(ctx, "vault-dynamic-configuration-operator-lock")
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	mgr, err := manager.New(cfg, manager.Options{
		Namespace:          namespace,
		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
	})
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
	if err := bankvaultsv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	if err := controller.AddToManager(mgr); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	if err = serveCRMetrics(cfg); err != nil {
		log.Info("Could not generate and serve custom resource metrics", "error", err.Error())
	}

	servicePorts := []v1.ServicePort{
		{Port: metricsPort, Name: metrics.OperatorPortName, Protocol: v1.ProtocolTCP, TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: metricsPort}},
		{Port: operatorMetricsPort, Name: metrics.CRPortName, Protocol: v1.ProtocolTCP, TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: operatorMetricsPort}},
	}
	service, err := metrics.CreateMetricsService(ctx, cfg, servicePorts)
	if err != nil {
		log.Info("Could not create metrics Service", "error", err.Error())
	}

	services := []*v1.Service{service}
	operatorNs, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		log.Info("Could not get operator namespace", "error", err.Error())
	}
	_, err = metrics.CreateServiceMonitors(cfg, operatorNs, services)
	if err != nil {
		log.Info("Could not create ServiceMonitor object", "error", err.Error())
		if err == metrics.ErrServiceMonitorNotPresent {
			log.Info("Install prometheus-operator in your cluster to create ServiceMonitor objects", "error", err.Error())
		}
	}

	log.Info("Starting the Cmd.")

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}

func serveCRMetrics(cfg *rest.Config) error {
	filteredGVK, err := k8sutil.GetGVKsFromAddToScheme(apis.AddToScheme)
	if err != nil {
		return err
	}
	operatorNs, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		return err
	}
	ns := []string{operatorNs}
	err = kubemetrics.GenerateAndServeCRMetrics(cfg, ns, filteredGVK, metricsHost, operatorMetricsPort)
	if err != nil {
		return err
	}
	return nil
}
