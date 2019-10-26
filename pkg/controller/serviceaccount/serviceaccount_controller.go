package serviceaccount

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	bankvaultsv1alpha1 "github.com/banzaicloud/bank-vaults/operator/pkg/apis/vault/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const policyTemplate = "path \"secret/%s\" { capabilities = [\"create\", \"read\", \"update\", \"delete\", \"list\"] }"

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

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner ServiceAccount
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &corev1.ServiceAccount{},
	})
	if err != nil {
		return err
	}

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

	if _, ok := instance.Annotations["vault.patoarvizu.dev/auto-configure"]; !ok {
		reqLogger.Info("Service account not annotated", "ServiceAccount", instance.ObjectMeta.Name)
		return reconcile.Result{}, nil
	}

	// Define a new Pod object
	pod := newPodForCR(instance)

	// Set ServiceAccount instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
		return reconcile.Result{}, err
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
			reqLogger.Info("Bank Vaults config", "Type:", reflect.TypeOf(bvConfig), "Config:", bvConfig)
			reqLogger.Info("Bank Vaults roles", "Roles", bvConfig.Auth[0].Roles)
			if !roleExists(bvConfig.Auth[0].Roles, instance.ObjectMeta.Name+"-role") {
				newPolicy := &policy{
					Name:  instance.ObjectMeta.Name + "-policy",
					Rules: fmt.Sprintf(policyTemplate, instance.ObjectMeta.Name),
				}
				bvConfig.Policies = append(bvConfig.Policies, *newPolicy)
				newRole := &role{
					BoundServiceAccountNames:      instance.ObjectMeta.Name,
					BoundServiceAccountNamespaces: instance.ObjectMeta.Namespace,
					Name:                          instance.ObjectMeta.Name + "-role",
					Policies:                      []string{instance.ObjectMeta.Name + "-policy"},
				}
				bvConfig.Auth[0].Roles = append(bvConfig.Auth[0].Roles, *newRole)
				configJsonData, _ := json.Marshal(bvConfig)
				reqLogger.Info("Config JSON Data", "JSON", configJsonData)
				err = json.Unmarshal(configJsonData, &vaultConfig.Spec.ExternalConfig)
				reqLogger.Info("Updated external config", "Updated config", vaultConfig.Spec.ExternalConfig)
				if err != nil {
					reqLogger.Error(err, "Error unmarshaling updated config")
				} else {
					r.client.Update(context.TODO(), vaultConfig)
				}
			}
		}
		// auth, _ := vaultConfig.Spec.ExternalConfig["auth"].([]interface{})
		// roles, _ := vaultConfig.Spec.ExternalConfig["auth"].([]interface{})[0].(map[string]interface{})["roles"]
		// reqLogger.Info("Vault config", "Type:", reflect.TypeOf(auth), "Config:", auth)
		// reqLogger.Info("Roles config", "Type:", reflect.TypeOf(roles), "Config:", roles)
	}

	// Check if this Pod already exists
	found := &corev1.Pod{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
		err = r.client.Create(context.TODO(), pod)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Pod created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Pod already exists - don't requeue
	reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
	return reconcile.Result{}, nil
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
