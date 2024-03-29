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

package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"text/template"

	bankvaultsv1alpha1 "github.com/banzaicloud/bank-vaults/operator/pkg/apis/vault/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
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

var log = logf.Log.WithName("controller_vdc")

const defaultPolicyTemplate = "path \"secret/{{ .Name }}\" { capabilities = [\"read\"] }"
const defaultDynamicDBUserCreationStatement = "CREATE USER '{{name}}'@'%' IDENTIFIED BY '{{password}}'; GRANT ALL ON *.* TO '{{name}}'@'%';"
const defaultDbDefaultTtl = "1h"
const defaultDbMaxTtl = "24h"

type BankVaultsConfig struct {
	Auth     []Auth   `json:"auth"`
	Policies []Policy `json:"policies"`
	Secrets  []Secret `json:"secrets,omitempty"`
}

type Auth struct {
	Roles  []Role                 `json:"roles"`
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config,omitempty"`
}

type Policy struct {
	Name  string `json:"name"`
	Rules string `json:"rules"`
}

type Secret struct {
	Type          string          `json:"type"`
	Configuration DBConfiguration `json:"configuration"`
}

type DBConfiguration struct {
	Config []DBConfig `json:"config"`
	Roles  []DBRole   `json:"roles"`
}

type DBConfig struct {
	Name                  string   `json:"name"`
	PluginName            string   `json:"plugin_name"`
	MaxOpenConnections    int      `json:"max_open_connections,omitempty"`
	MaxIdleConnections    int      `json:"max_idle_connections,omitempty"`
	MaxConnectionLifetime string   `json:"max_connection_lifetime,omitempty"`
	ConnectionUrl         string   `json:"connection_url"`
	AllowedRoles          []string `json:"allowed_roles"`
	Username              string   `json:"username"`
	Password              string   `json:"password"`
}

type DBRole struct {
	Name               string   `json:"name"`
	DbName             string   `json:"db_name"`
	CreationStatements []string `json:"creation_statements"`
	DefaultTtl         string   `json:"default_ttl,omitempty"`
	MaxTtl             string   `json:"max_ttl,omitempty"`
}

type Role struct {
	BoundServiceAccountNames      string      `json:"bound_service_account_names"`
	BoundServiceAccountNamespaces interface{} `json:"bound_service_account_namespaces"`
	Name                          string      `json:"name"`
	TokenPolicies                 []string    `json:"token_policies"`
	TokenTtl                      string      `json:"token_ttl,omitempty"`
	TokenMaxTtl                   string      `json:"token_max_ttl,omitempty"`
	TokenBoundCidrs               []string    `json:"token_bound_cidrs,omitempty"`
	TokenExplicitMaxTtl           string      `json:"token_explicit_max_ttl,omitempty"`
	TokenNoDefaultPolicy          bool        `json:"token_no_default_policy,omitempty"`
	TokenNumUses                  int         `json:"token_num_uses,omitempty"`
	TokenPeriod                   string      `json:"token_period,omitempty"`
	TokenType                     string      `json:"token_type,omitempty"`
}

type policyTemplateInput struct {
	Name      string
	Namespace string
}

// ServiceAccountReconciler reconciles a ServiceAccount object
type ServiceAccountReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=vault.banzaicloud.com,resources=vaults,verbs=get;list;watch;create;update;patch

func (r *ServiceAccountReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)

	instance := &corev1.ServiceAccount{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if val, ok := instance.Annotations[AnnotationPrefix+"/"+AutoConfigureAnnotation]; !ok || val != "true" {
		return reconcile.Result{}, nil
	}

	if instance.ObjectMeta.Name == "default" {
		reqLogger.V(1).Info(fmt.Sprintf("Explicitly ignoring 'default' ServiceAccount in namespace %s, to avoid overwriting Vaults 'default' policy", instance.ObjectMeta.Namespace))
		return reconcile.Result{}, nil
	}

	vaultConfig := &bankvaultsv1alpha1.Vault{}
	ns, _ := getOperatorNamespace()
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: TargetVaultName, Namespace: ns}, vaultConfig)
	if err != nil {
		return reconcile.Result{}, err
	}
	var bvConfig BankVaultsConfig
	jsonData, _ := json.Marshal(vaultConfig.Spec.ExternalConfig)
	err = json.Unmarshal(jsonData, &bvConfig)
	if err != nil {
		return reconcile.Result{}, err
	}
	configMap := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "vault-dynamic-configuration", Namespace: "vault"}, configMap)
	if err != nil {
		reqLogger.V(1).Info("vault-dynamic-configuration ConfigMap not found, using defaults")
	}
	err = addOrUpdatePolicy(&bvConfig, instance.ObjectMeta, *configMap)
	if err != nil {
		return reconcile.Result{}, err
	}
	kubernetesAuth, err := bvConfig.getKubernetesAuth()
	if err != nil {
		return reconcile.Result{}, err
	}
	addOrUpdateKubernetesRole(kubernetesAuth, instance.ObjectMeta)
	reqLogger.V(1).Info("Added Kubernetes role")
	err = updateKubernetesConfiguration(bvConfig, vaultConfig)
	if err != nil {
		return reconcile.Result{}, err
	}
	r.Client.Update(context.TODO(), vaultConfig)

	targetDb, ok := instance.Annotations[AnnotationPrefix+"/"+DynamicDBCredentialsAnnotation]
	if !ok {
		return reconcile.Result{}, nil
	}
	err = addOrUpdateDBRole(&bvConfig, instance.ObjectMeta, *configMap, targetDb)
	if err != nil {
		return reconcile.Result{}, err
	}
	err = updateDBSecretConfiguration(bvConfig, vaultConfig)
	if err != nil {
		return reconcile.Result{}, err
	}
	r.Client.Update(context.TODO(), vaultConfig)
	return reconcile.Result{}, nil
}

func (r *ServiceAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New("serviceaccount-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.ServiceAccount{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{
		Type: &bankvaultsv1alpha1.Vault{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(h handler.MapObject) []reconcile.Request {
				return getRequestsForAllAnnotatedServiceAccounts(mgr)
			}),
		},
	)
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{
		Type: &corev1.ConfigMap{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(h handler.MapObject) []reconcile.Request {
				return getRequestsForAllAnnotatedServiceAccounts(mgr)
			}),
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func getOperatorNamespace() (string, error) {
	nsBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", err
	}
	ns := strings.TrimSpace(string(nsBytes))
	return ns, nil
}

func getRequestsForAllAnnotatedServiceAccounts(mgr manager.Manager) []reconcile.Request {
	namespaces := &corev1.NamespaceList{}
	mgr.GetClient().List(context.TODO(), namespaces)
	requests := []reconcile.Request{}
	for _, ns := range namespaces.Items {
		serviceAccounts := &corev1.ServiceAccountList{}
		mgr.GetClient().List(context.TODO(), serviceAccounts, client.InNamespace(ns.ObjectMeta.Name))
		for _, sa := range serviceAccounts.Items {
			if val, ok := sa.ObjectMeta.Annotations[AnnotationPrefix+"/"+AutoConfigureAnnotation]; ok {
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
}

func addOrUpdateDBRole(bvConfig *BankVaultsConfig, metadata metav1.ObjectMeta, configMap corev1.ConfigMap, targetDb string) error {
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

	dbSecret, err := bvConfig.GetDBSecret()
	if err != nil {
		return err
	}
	for _, r := range dbSecret.Configuration.Roles {
		if r.Name == metadata.Name {
			return nil
		}
	}
	log.V(1).Info("Configuring ServiceAccount for dynamic database secrets", "ServiceAccount", metadata.Name, "Namespace", metadata.Namespace, "TargetDB", targetDb)
	newDbRole := &DBRole{
		Name:               metadata.Name,
		DbName:             targetDb,
		CreationStatements: []string{creationStatement},
		DefaultTtl:         dbDefaultTtl,
		MaxTtl:             dbMaxTtl,
	}
	dbConfig, err := dbSecret.Configuration.GetDBConfig(targetDb)
	if err != nil {
		return err
	}
	dbConfig.AllowedRoles = append(dbConfig.AllowedRoles, metadata.Name)
	dbSecret.Configuration.Roles = append(dbSecret.Configuration.Roles, *newDbRole)
	return nil
}

func addOrUpdatePolicy(bvConfig *BankVaultsConfig, metadata metav1.ObjectMeta, configMap corev1.ConfigMap) error {
	var policyTemplate string
	if val, ok := configMap.Data["policy-template"]; !ok {
		policyTemplate = defaultPolicyTemplate
	} else {
		policyTemplate = val
	}
	t := template.Must(template.New("policy").Parse(policyTemplate))
	var parsedBuffer bytes.Buffer
	t.Execute(&parsedBuffer, policyTemplateInput{
		Name:      metadata.Name,
		Namespace: metadata.Namespace,
	})
	for i, r := range bvConfig.Policies {
		if r.Name == metadata.Name {
			bvConfig.Policies[i].Rules = parsedBuffer.String()
			return nil
		}
	}
	newPolicy := &Policy{
		Name:  metadata.Name,
		Rules: parsedBuffer.String(),
	}
	bvConfig.Policies = append(bvConfig.Policies, *newPolicy)
	return nil
}

func addOrUpdateKubernetesRole(kubernetesAuth *Auth, metadata metav1.ObjectMeta) {
	for i, r := range kubernetesAuth.Roles {
		if r.Name == metadata.Name {
			if BoundRolesToAllNamespaces {
				kubernetesAuth.Roles[i].BoundServiceAccountNamespaces = []string{"*"}
			} else {
				switch r.BoundServiceAccountNamespaces.(type) {
				case string:
					kubernetesAuth.Roles[i].BoundServiceAccountNamespaces = []string{metadata.Name}
				case []interface{}:
					for _, n := range r.BoundServiceAccountNamespaces.([]interface{}) {
						if n.(string) == metadata.Namespace {
							return
						}
					}
					kubernetesAuth.Roles[i].BoundServiceAccountNamespaces = append(kubernetesAuth.Roles[i].BoundServiceAccountNamespaces.([]interface{}), metadata.Namespace)
				}
			}
			return
		}
	}
	log.V(1).Info("Configuring ServiceAccount for Vault authentication", "ServiceAccount", metadata.Name, "Namespace", metadata.Namespace)
	newRole := &Role{
		BoundServiceAccountNames: metadata.Name,
		BoundServiceAccountNamespaces: func(namespace string) []string {
			if BoundRolesToAllNamespaces {
				return []string{"*"}
			} else {
				return []string{namespace}
			}
		}(metadata.Namespace),
		Name:          metadata.Name,
		TokenPolicies: []string{metadata.Name},
		TokenTtl:      TokenTtl,
	}
	kubernetesAuth.Roles = append(kubernetesAuth.Roles, *newRole)
}

func updateDBSecretConfiguration(bvConfig BankVaultsConfig, vaultConfig *bankvaultsv1alpha1.Vault) error {
	jsonMap := make(map[string]interface{})
	err := json.Unmarshal([]byte(vaultConfig.Spec.ExternalConfigJSON()), &jsonMap)
	if err != nil {
		return err
	}
	dbSecret, err := bvConfig.GetDBSecret()
	if err != nil {
		return err
	}
	configJsonData, err := json.Marshal(dbSecret)
	for i, s := range bvConfig.Secrets {
		if s.Type != "database" {
			continue
		}
		err = json.Unmarshal(configJsonData, &jsonMap["secrets"].([]interface{})[i])
		if err != nil {
			return err
		}
		unmarshaledJsonMap, err := json.Marshal(jsonMap)
		if err != nil {
			return err
		}
		vaultConfig.Spec.ExternalConfig.Raw = unmarshaledJsonMap
		return nil
	}
	return nil
}

func updateKubernetesConfiguration(bvConfig BankVaultsConfig, vaultConfig *bankvaultsv1alpha1.Vault) error {
	jsonMap := make(map[string]interface{})
	err := json.Unmarshal([]byte(vaultConfig.Spec.ExternalConfigJSON()), &jsonMap)
	if err != nil {
		return err
	}
	kubernetesAuth, err := bvConfig.getKubernetesAuth()
	if err != nil {
		return err
	}
	configJsonData, err := json.Marshal(kubernetesAuth)
	if err != nil {
		return err
	}
	for i, a := range bvConfig.Auth {
		if a.Type != "kubernetes" {
			continue
		}
		err = json.Unmarshal(configJsonData, &jsonMap["auth"].([]interface{})[i])
		if err != nil {
			return err
		}
		jsonMap["policies"] = bvConfig.Policies
		unmarshaledJsonMap, err := json.Marshal(jsonMap)
		if err != nil {
			return err
		}
		vaultConfig.Spec.ExternalConfig.Raw = unmarshaledJsonMap
		return nil
	}
	return nil
}

func (bvConfig BankVaultsConfig) GetDBSecret() (*Secret, error) {
	for i, s := range bvConfig.Secrets {
		if s.Type == "database" {
			return &bvConfig.Secrets[i], nil
		}
	}
	return &Secret{}, errors.New("Database secrets configuration not found")
}

func (dbConfiguration DBConfiguration) GetDBConfig(targetDb string) (*DBConfig, error) {
	for i, c := range dbConfiguration.Config {
		if c.Name == targetDb {
			return &dbConfiguration.Config[i], nil
		}
	}
	return &DBConfig{}, errors.New(fmt.Sprintf("Database %s configuration not found", targetDb))
}

func (bvConfig BankVaultsConfig) getKubernetesAuth() (*Auth, error) {
	for i, a := range bvConfig.Auth {
		if a.Type == "kubernetes" {
			return &bvConfig.Auth[i], nil
		}
	}
	return &Auth{}, errors.New("Kubernetes authentication configuration not found")
}

func (bvConfig BankVaultsConfig) GetRole(name string) (Role, error) {
	kubernetesAuth, err := bvConfig.getKubernetesAuth()
	if err != nil {
		return Role{}, err
	}
	for _, r := range kubernetesAuth.Roles {
		if r.Name == name {
			return r, nil
		}
	}
	return Role{}, errors.New(fmt.Sprintf("Role %s not found", name))
}

func (bvConfig BankVaultsConfig) GetDBRole(name string) (DBRole, error) {
	dbSecret, err := bvConfig.GetDBSecret()
	if err != nil {
		return DBRole{}, err
	}
	for _, r := range dbSecret.Configuration.Roles {
		if r.Name == name {
			return r, nil
		}
	}
	return DBRole{}, errors.New(fmt.Sprintf("Role %s not found", name))
}

func (bvConfig BankVaultsConfig) GetPolicy(name string) (Policy, error) {
	for _, p := range bvConfig.Policies {
		if p.Name == name {
			return p, nil
		}
	}
	return Policy{}, errors.New(fmt.Sprintf("Policy %s not found", name))
}
