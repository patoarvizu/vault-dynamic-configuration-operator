package serviceaccount

import (
	"bytes"
	"context"
	"encoding/json"
	"text/template"

	bankvaultsv1alpha1 "github.com/banzaicloud/bank-vaults/operator/pkg/apis/vault/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const defaultPolicyTemplate = "path \"secret/{{ .Name }}\" { capabilities = [\"read\"] }"

var log = logf.Log.WithName("controller_serviceaccount")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new ServiceAccount Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileServiceAccount{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("serviceaccount-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ServiceAccount
	err = c.Watch(&source.Kind{Type: &corev1.ServiceAccount{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{
		Type: &corev1.ConfigMap{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(h handler.MapObject) []reconcile.Request {
				namespaces := &corev1.NamespaceList{}
				mgr.GetClient().List(context.TODO(), namespaces)
				requests := []reconcile.Request{}
				for _, ns := range namespaces.Items {
					serviceAccounts := &corev1.ServiceAccountList{}
					mgr.GetClient().List(context.TODO(), serviceAccounts, client.InNamespace(ns.ObjectMeta.Name))
					for _, sa := range serviceAccounts.Items {
						if val, ok := sa.ObjectMeta.Annotations["vault.patoarvizu.dev/auto-configure"]; ok {
							if val == "true" {
								requests = append(requests, reconcile.Request{
									NamespacedName: types.NamespacedName{
										Name:      sa.ObjectMeta.Name,
										Namespace: sa.ObjectMeta.Namespace,
									},
								})
							}
						}
					}
				}
				return requests
			}),
		},
	)

	return nil
}

// blank assignment to verify that ReconcileServiceAccount implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileServiceAccount{}

// ReconcileServiceAccount reconciles a ServiceAccount object
type ReconcileServiceAccount struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

type bankVaultsConfig struct {
	Auth     []auth   `json:"auth"`
	Policies []policy `json:"policies"`
}

type auth struct {
	Roles []role `json:"roles"`
	Type  string `json:"type"`
}

type policy struct {
	Name  string `json:"name"`
	Rules string `json:"rules"`
}

type role struct {
	BoundServiceAccountNames      string   `json:"bound_service_account_names"`
	BoundServiceAccountNamespaces string   `json:"bound_service_account_namespaces"`
	Name                          string   `json:"name"`
	Policies                      []string `json:"policies"`
}

type policyTemplateInput struct {
	Name      string
	Namespace string
}

// Reconcile reads that state of the cluster for a ServiceAccount object and makes changes based on the state read
// and what is in the ServiceAccount.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileServiceAccount) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ServiceAccount")

	// Fetch the ServiceAccount instance
	instance := &corev1.ServiceAccount{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if val, ok := instance.Annotations["vault.patoarvizu.dev/auto-configure"]; !ok || val != "true" {
		reqLogger.Info("Service account not annotated or auto-configure set to 'false'", "ServiceAccount", instance.ObjectMeta.Name)
		return reconcile.Result{}, nil
	}

	vaultConfig := &bankvaultsv1alpha1.Vault{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "vault", Namespace: "vault"}, vaultConfig)
	if err != nil {
		reqLogger.Error(err, "Error getting Vault configuration")
	} else {
		var bvConfig bankVaultsConfig
		jsonData, _ := json.Marshal(vaultConfig.Spec.ExternalConfig)
		err = json.Unmarshal(jsonData, &bvConfig)
		if err != nil {
			reqLogger.Error(err, "Error unmarshaling config")
		} else {
			configMap := &corev1.ConfigMap{}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: "vault-dynamic-configuration", Namespace: "vault"}, configMap)
			if err != nil {
				reqLogger.Info("vault-dynamic-configuration ConfigMap not found, using defaults")
			}
			var policyTemplate string
			if val, ok := configMap.Data["policy-template"]; !ok {
				policyTemplate = defaultPolicyTemplate
			} else {
				policyTemplate = val
			}
			t := template.Must(template.New("policy").Parse(policyTemplate))
			var parsedBuffer bytes.Buffer
			t.Execute(&parsedBuffer, policyTemplateInput{
				Name:      instance.ObjectMeta.Name,
				Namespace: instance.ObjectMeta.Namespace,
			})
			kubernetesAuthIndex := getKubernetesAuthIndex(bvConfig)
			if !roleExists(bvConfig.Auth[kubernetesAuthIndex].Roles, instance.ObjectMeta.Name) {
				newPolicy := &policy{
					Name:  instance.ObjectMeta.Name,
					Rules: parsedBuffer.String(),
				}
				bvConfig.Policies = append(bvConfig.Policies, *newPolicy)
				newRole := &role{
					BoundServiceAccountNames:      instance.ObjectMeta.Name,
					BoundServiceAccountNamespaces: instance.ObjectMeta.Namespace,
					Name:                          instance.ObjectMeta.Name,
					Policies:                      []string{instance.ObjectMeta.Name},
				}
				bvConfig.Auth[kubernetesAuthIndex].Roles = append(bvConfig.Auth[kubernetesAuthIndex].Roles, *newRole)
			} else {
				existingPolicyIndex := getExistingPolicyIndex(bvConfig.Policies, instance.ObjectMeta.Name)
				bvConfig.Policies[existingPolicyIndex].Rules = parsedBuffer.String()
			}
			configJsonData, _ := json.Marshal(bvConfig)
			err = json.Unmarshal(configJsonData, &vaultConfig.Spec.ExternalConfig)
			if err != nil {
				reqLogger.Error(err, "Error unmarshaling updated config")
			} else {
				r.client.Update(context.TODO(), vaultConfig)
			}
		}
	}

	return reconcile.Result{}, nil
}

func getKubernetesAuthIndex(bvConfig bankVaultsConfig) int {
	for i, a := range bvConfig.Auth {
		if a.Type == "kubernetes" {
			return i
		}
	}
	return -1
}

func getExistingPolicyIndex(policies []policy, name string) int {
	for i, p := range policies {
		if p.Name == name {
			return i
		}
	}
	return -1
}

func roleExists(roles []role, name string) bool {
	for _, r := range roles {
		if r.Name == name {
			return true
		}
	}
	return false
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *corev1.ServiceAccount) *corev1.Pod {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-pod",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}
}
