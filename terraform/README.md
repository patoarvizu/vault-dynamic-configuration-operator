<!-- BEGIN_TF_DOCS -->

## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 0.14.9 |
| <a name="requirement_kubernetes"></a> [kubernetes](#requirement\_kubernetes) | ~> 2.8.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_kubernetes"></a> [kubernetes](#provider\_kubernetes) | ~> 2.8.0 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [kubernetes_cluster_role_binding_v1.cluster_role_binding](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/cluster_role_binding_v1) | resource |
| [kubernetes_cluster_role_v1.cluster_role](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/cluster_role_v1) | resource |
| [kubernetes_config_map_v1.configmap](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/config_map_v1) | resource |
| [kubernetes_deployment_v1.deployment](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/deployment_v1) | resource |
| [kubernetes_manifest.servicemonitor_metrics](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/manifest) | resource |
| [kubernetes_namespace_v1.ns](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/namespace_v1) | resource |
| [kubernetes_role_binding_v1.rolebinding](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/role_binding_v1) | resource |
| [kubernetes_role_v1.role](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/role_v1) | resource |
| [kubernetes_service_account_v1.sa](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/service_account_v1) | resource |
| [kubernetes_service_v1.metrics](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/service_v1) | resource |
| [kubernetes_namespace.ns](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/data-sources/namespace) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_create_namespace"></a> [create\_namespace](#input\_create\_namespace) | If true, a new namespace will be created with the name set to the value of the namespace\_name variable. If false, it will look up an existing namespace with the name of the value of the namespace\_name variable. | `bool` | `false` | no |
| <a name="input_db_default_ttl"></a> [db\_default\_ttl](#input\_db\_default\_ttl) | The default value of the `db-default-ttl` setting | `string` | `"1h"` | no |
| <a name="input_db_max_ttl"></a> [db\_max\_ttl](#input\_db\_max\_ttl) | The default value of the `db-max-ttl` setting | `string` | `"24h"` | no |
| <a name="input_db_user_creation_statement"></a> [db\_user\_creation\_statement](#input\_db\_user\_creation\_statement) | The default value of the `db-user-creation-statement` setting | `string` | `"CREATE USER '{{name}}'@'%' IDENTIFIED BY '{{password}}'; GRANT ALL ON *.* TO '{{name}}'@'%';"` | no |
| <a name="input_enable_prometheus_monitoring"></a> [enable\_prometheus\_monitoring](#input\_enable\_prometheus\_monitoring) | Create the `Service` and `ServiceMonitor` objects to enable Prometheus monitoring on the operator. | `bool` | `false` | no |
| <a name="input_flag_annotation_prefix"></a> [flag\_annotation\_prefix](#input\_flag\_annotation\_prefix) | The value of the --annotation-prefix flag | `string` | `"vault.patoarvizu.dev"` | no |
| <a name="input_flag_auto_configure_annotation"></a> [flag\_auto\_configure\_annotation](#input\_flag\_auto\_configure\_annotation) | The value of the --auto-configure-annotation flag | `string` | `"auto-configure"` | no |
| <a name="input_flag_auto_configure_db_creds_annotation"></a> [flag\_auto\_configure\_db\_creds\_annotation](#input\_flag\_auto\_configure\_db\_creds\_annotation) | The value of the --auto-configuredb-creds-annotation flag | `string` | `"db-dynamic-creds"` | no |
| <a name="input_flag_bound_roles_to_all_namespaces"></a> [flag\_bound\_roles\_to\_all\_namespaces](#input\_flag\_bound\_roles\_to\_all\_namespaces) | The value of the --bound-roles-to-all-namespaces flag | `bool` | `false` | no |
| <a name="input_flag_target_vault_name"></a> [flag\_target\_vault\_name](#input\_flag\_target\_vault\_name) | The value of the --target-vault-name flag | `string` | `"vault"` | no |
| <a name="input_flag_token_ttl"></a> [flag\_token\_ttl](#input\_flag\_token\_ttl) | The value of the --token-ttl flag | `string` | `"5m"` | no |
| <a name="input_image_version"></a> [image\_version](#input\_image\_version) | The label of the image to run. | `string` | `"latest"` | no |
| <a name="input_namespace_name"></a> [namespace\_name](#input\_namespace\_name) | The name of the namespace to create or look up. | `string` | `"vault"` | no |
| <a name="input_policy_template"></a> [policy\_template](#input\_policy\_template) | The default value of the `policy-template` setting | `string` | `"path \"secret/{{ .Name }}\" {\n  capabilities = [\"read\"]\n}\n"` | no |
| <a name="input_service_account_name"></a> [service\_account\_name](#input\_service\_account\_name) | The name of the service account to create. | `string` | `"vault-dynamic-configuration-operator"` | no |
| <a name="input_service_monitor_custom_labels"></a> [service\_monitor\_custom\_labels](#input\_service\_monitor\_custom\_labels) | Custom labels to add to the `ServiceMonitor` object. | `map` | `{}` | no |
| <a name="input_watch_namespace"></a> [watch\_namespace](#input\_watch\_namespace) | The value to be set on the `WATCH_NAMESPACE` environment variable. | `string` | `""` | no |

## Outputs

No outputs.
<!-- END_TF_DOCS -->