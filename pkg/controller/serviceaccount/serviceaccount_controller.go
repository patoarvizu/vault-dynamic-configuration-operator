package serviceaccount

import (
	"bytes"
	"context"
	"encoding/json"
	"text/template"

	bankvaultsv1alpha1 "github.com/banzaicloud/bank-vaults/operator/pkg/apis/vault/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileServiceAccount{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New("serviceaccount-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

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
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileServiceAccount{}

type ReconcileServiceAccount struct {
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

func (r *ReconcileServiceAccount) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ServiceAccount")

	instance := &corev1.ServiceAccount{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
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
		return reconcile.Result{}, err
	} else {
		var bvConfig bankVaultsConfig
		jsonData, _ := json.Marshal(vaultConfig.Spec.ExternalConfig)
		err = json.Unmarshal(jsonData, &bvConfig)
		if err != nil {
			reqLogger.Error(err, "Error unmarshaling config")
			return reconcile.Result{}, err
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
				return reconcile.Result{}, err
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
