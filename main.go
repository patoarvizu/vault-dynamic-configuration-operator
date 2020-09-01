/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/gob"
	"flag"
	"os"

	bankvaultsv1alpha1 "github.com/banzaicloud/bank-vaults/operator/pkg/apis/vault/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/patoarvizu/vault-dynamic-configuration-operator/controllers"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(corev1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&controllers.TargetVaultName, "target-vault-name", "vault", "Name of Vault custom resource to target")
	flag.StringVar(&controllers.AnnotationPrefix, "annotation-prefix", "vault.patoarvizu.dev", "Prefix of the annotations the operator should watch for in service accounts to configure roles and policies")
	flag.StringVar(&controllers.AutoConfigureAnnotation, "auto-configure-annotation", "auto-configure", "Annotation the operator should watch for in service accounts")
	flag.StringVar(&controllers.DynamicDBCredentialsAnnotation, "auto-configuredb-creds-annotation", "db-dynamic-creds", "Annotation the operator should watch for in service accounts to configure access to dynamic DB credentials")
	flag.BoolVar(&controllers.BoundRolesToAllNamespaces, "bound-roles-to-all-namespaces", false, "Set 'bound_service_account_namespaces' to '*' instead of the service account's namespace")
	flag.StringVar(&controllers.TokenTtl, "token-ttl", "5m", "Value to set roles' 'token_ttl' to")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(false)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "a5d7539a.my.domain",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
	if err := bankvaultsv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "Error registering bank-vaults scheme")
		os.Exit(1)
	}

	if err = (&controllers.ServiceAccountReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("ServiceAccount"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ServiceAccount")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
