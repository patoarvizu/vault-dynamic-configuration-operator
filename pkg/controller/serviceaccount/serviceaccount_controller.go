package serviceaccount

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"text/template"

	bankvaultsv1alpha1 "github.com/banzaicloud/bank-vaults/operator/pkg/apis/vault/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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

var (
	TargetVaultName                string
	AnnotationPrefix               string
	AutoConfigureAnnotation        string
	DynamicDBCredentialsAnnotation string
	BoundRolesToAllNamespaces      bool
	TokenTtl                       string
)

const defaultPolicyTemplate = "path \"secret/{{ .Name }}\" { capabilities = [\"read\"] }"
const defaultDynamicDBUserCreationStatement = "CREATE USER '{{name}}'@'%' IDENTIFIED BY '{{password}}'; GRANT ALL ON *.* TO '{{name}}'@'%';"
const defaultDbDefaultTtl = "1h"
const defaultDbMaxTtl = "24h"

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
						if val, ok := sa.ObjectMeta.Annotations[AutoConfigureAnnotation]; ok {
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
	Secrets  []secret `json:"secrets,omitempty"`
}

type auth struct {
	Roles []role `json:"roles"`
	Type  string `json:"type"`
}

type policy struct {
	Name  string `json:"name"`
	Rules string `json:"rules"`
}

type secret struct {
	Type          string          `json:"type"`
	Configuration dbConfiguration `json:"configuration"`
}

type dbConfiguration struct {
	Config []dbConfig `json:"config"`
	Roles  []dbRole   `json:"roles"`
}

type dbConfig struct {
	Name                  string      `json:"name"`
	PluginName            string      `json:"plugin_name"`
	MaxOpenConnections    int         `json:"max_open_connections,omitempty"`
	MaxIdleConnections    int         `json:"max_idle_connections,omitempty"`
	MaxConnectionLifetime string      `json:"max_connection_lifetime,omitempty"`
	ConnectionUrl         string      `json:"connection_url"`
	AllowedRoles          interface{} `json:"allowed_roles"`
	Username              string      `json:"username"`
	Password              string      `json:"password"`
}

type dbRole struct {
	Name               string   `json:"name"`
	DbName             string   `json:"db_name"`
	CreationStatements []string `json:"creation_statements"`
	DefaultTtl         string   `json:"default_ttl,omitempty"`
	MaxTtl             string   `json:"max_ttl,omitempty"`
}

type role struct {
	BoundServiceAccountNames      string   `json:"bound_service_account_names"`
	BoundServiceAccountNamespaces string   `json:"bound_service_account_namespaces"`
	Name                          string   `json:"name"`
	TokenPolicies                 []string `json:"token_policies"`
	TokenTtl                      string   `json:"token_ttl,omitempty"`
	TokenMaxTtl                   string   `json:"token_max_ttl,omitempty"`
	TokenBoundCidrs               []string `json:"token_bound_cidrs,omitempty"`
	TokenExplicitMaxTtl           string   `json:"token_explicit_max_ttl,omitempty"`
	TokenNoDefaultPolicy          bool     `json:"token_no_default_policy,omitempty"`
	TokenNumUses                  int      `json:"token_num_uses,omitempty"`
	TokenPeriod                   string   `json:"token_period,omitempty"`
	TokenType                     string   `json:"token_type,omitempty"`
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
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if val, ok := instance.Annotations[AnnotationPrefix+"/"+AutoConfigureAnnotation]; !ok || val != "true" {
		reqLogger.Info("Service account not annotated or auto-configure set to 'false'", "ServiceAccount", instance.ObjectMeta.Name)
		return reconcile.Result{}, nil
	}

	vaultConfig := &bankvaultsv1alpha1.Vault{}
	ns, _ := k8sutil.GetOperatorNamespace()
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: TargetVaultName, Namespace: ns}, vaultConfig)
	if err != nil {
		reqLogger.Error(err, "Error getting Vault configuration")
		return reconcile.Result{}, err
	}
	var bvConfig bankVaultsConfig
	jsonData, _ := json.Marshal(vaultConfig.Spec.ExternalConfig)
	err = json.Unmarshal(jsonData, &bvConfig)
	if err != nil {
		reqLogger.Error(err, "Error unmarshaling config")
		return reconcile.Result{}, err
	}
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
	kubernetesAuthIndex, err := getKubernetesAuthIndex(bvConfig)
	if err != nil {
		reqLogger.Error(err, "Can't find kubernetes auth configuration")
		return reconcile.Result{}, err
	}
	if !policyExists(bvConfig.Policies, instance.ObjectMeta.Name) {
		newPolicy := &policy{
			Name:  instance.ObjectMeta.Name,
			Rules: parsedBuffer.String(),
		}
		bvConfig.Policies = append(bvConfig.Policies, *newPolicy)
	} else {
		existingPolicyIndex := getExistingPolicyIndex(bvConfig.Policies, instance.ObjectMeta.Name)
		bvConfig.Policies[existingPolicyIndex].Rules = parsedBuffer.String()
	}
	if !roleExists(bvConfig.Auth[kubernetesAuthIndex].Roles, instance.ObjectMeta.Name) {
		newRole := &role{
			BoundServiceAccountNames: instance.ObjectMeta.Name,
			BoundServiceAccountNamespaces: func(namespace string) string {
				if BoundRolesToAllNamespaces {
					return "*"
				} else {
					return namespace
				}
			}(instance.ObjectMeta.Namespace),
			Name:          instance.ObjectMeta.Name,
			TokenPolicies: []string{instance.ObjectMeta.Name},
			TokenTtl:      TokenTtl,
		}
		bvConfig.Auth[kubernetesAuthIndex].Roles = append(bvConfig.Auth[kubernetesAuthIndex].Roles, *newRole)
	}
	configJsonData, _ := json.Marshal(bvConfig)
	err = json.Unmarshal(configJsonData, &vaultConfig.Spec.ExternalConfig)
	if err != nil {
		reqLogger.Error(err, "Error unmarshaling updated config")
		return reconcile.Result{}, err
	}
	r.client.Update(context.TODO(), vaultConfig)

	targetDb, ok := instance.Annotations[AnnotationPrefix+"/"+DynamicDBCredentialsAnnotation]
	if !ok {
		reqLogger.Info("Service account not annotated for dynamic database credentials", "ServiceAccount", instance.ObjectMeta.Name)
		return reconcile.Result{}, nil
	}

	var creationStatement string
	if val, ok := configMap.Data["db-user-creation-statement"]; !ok {
		creationStatement = defaultDynamicDBUserCreationStatement
	} else {
		creationStatement = val
	}

	var dbDefaultTtl string
	if val, ok := configMap.Data["db-default-ttl"]; !ok {
		dbDefaultTtl = defaultDbDefaultTtl
	} else {
		dbDefaultTtl = val
	}

	var dbMaxTtl string
	if val, ok := configMap.Data["db-max-ttl"]; !ok {
		dbMaxTtl = defaultDbMaxTtl
	} else {
		dbMaxTtl = val
	}

	dbSecretsIndex, err := getDBSecretsIndex(bvConfig)
	if err != nil {
		reqLogger.Error(err, "Can't find database secrets configuration")
		return reconcile.Result{}, err
	}

	if !dbRoleExists(bvConfig.Secrets[dbSecretsIndex].Configuration.Roles, instance.ObjectMeta.Name) {
		newDbRole := &dbRole{
			Name:               instance.ObjectMeta.Name,
			DbName:             targetDb,
			CreationStatements: []string{creationStatement},
			DefaultTtl:         dbDefaultTtl,
			MaxTtl:             dbMaxTtl,
		}
		dbConfigIndex, err := getDbConfigIndex(bvConfig.Secrets[dbSecretsIndex], targetDb)
		if err != nil {
			reqLogger.Error(err, "Can'ttarget database secrets configuration")
			return reconcile.Result{}, err
		}
		bvConfig.Secrets[dbSecretsIndex].Configuration.Config[dbConfigIndex].AllowedRoles = append(bvConfig.Secrets[dbSecretsIndex].Configuration.Config[dbConfigIndex].AllowedRoles.([]interface{}), instance.ObjectMeta.Name)
		bvConfig.Secrets[dbSecretsIndex].Configuration.Roles = append(bvConfig.Secrets[dbSecretsIndex].Configuration.Roles, *newDbRole)
	}

	configJsonData, err = json.Marshal(bvConfig)
	if err != nil {
		reqLogger.Error(err, "Error marshaling updated config")
		return reconcile.Result{}, err
	}
	err = json.Unmarshal(configJsonData, &vaultConfig.Spec.ExternalConfig)
	if err != nil {
		reqLogger.Error(err, "Error unmarshaling updated config")
		return reconcile.Result{}, err
	}
	r.client.Update(context.TODO(), vaultConfig)

	return reconcile.Result{}, nil
}

func getBoundServiceAccountNamespace(namespace string) string {
	if BoundRolesToAllNamespaces {
		return "*"
	} else {
		return namespace
	}
}

func getDBSecretsIndex(bvConfig bankVaultsConfig) (int, error) {
	for i, s := range bvConfig.Secrets {
		if s.Type == "database" {
			return i, nil
		}
	}
	return -1, errors.New("Database secrets configuration not found")
}

func getDbConfigIndex(dbSecret secret, targetDb string) (int, error) {
	for i, c := range dbSecret.Configuration.Config {
		if c.Name == targetDb {
			return i, nil
		}
	}
	return -1, errors.New("Database configuration not found")
}

func getKubernetesAuthIndex(bvConfig bankVaultsConfig) (int, error) {
	for i, a := range bvConfig.Auth {
		if a.Type == "kubernetes" {
			return i, nil
		}
	}
	return -1, errors.New("Kubernetes authentication configuration not found")
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

func dbRoleExists(dbRoles []dbRole, name string) bool {
	for _, r := range dbRoles {
		if r.Name == name {
			return true
		}
	}
	return false
}

func policyExists(policies []policy, name string) bool {
	for _, r := range policies {
		if r.Name == name {
			return true
		}
	}
	return false
}
