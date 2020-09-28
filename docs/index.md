# vault-dynamic-configuration-operator

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square)

Vault dynamic configuration operator

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| defaultConfiguration | object | `{"dbDefaultTTL":"1h","dbMaxTTL":"24h","dbUserCreationStatement":"CREATE USER '{{name}}'@'%' IDENTIFIED BY '{{password}}'; GRANT ALL ON *.* TO '{{name}}'@'%';","policyTemplate":"path \"secret/{{ .Name }}\" {\n  capabilities = [\"read\"]\n}\n"}` | The values to be used for the default `vault-dynamic-configuration` `ConfigMap`. |
| defaultConfiguration.policyTemplate | string | `"path \"secret/{{ .Name }}\" {\n  capabilities = [\"read\"]\n}\n"` | Corresponds to the `policy-template` field of the default `ConfigMap`. |
| flags.annotationPrefix | string | `"vault.patoarvizu.dev"` | The value to be set on the `--annotation-prefix` flag. |
| flags.autoConfigureAnnotation | string | `"auto-configure"` |  |
| flags.autoConfigureDBCredsAnnotation | string | `"db-dynamic-creds"` | The value to be set on the `--auto-configuredb-creds-annotation` flag. |
| flags.boundRolesToAllNamespaces | bool | `false` | If set to `true` the `--bound-roles-to-all-namespaces` flag will be set. |
| flags.targetVaultName | string | `"vault"` | The value to be set on the `--target-vault-name` flag. |
| flags.tokenTTL | string | `"5m"` | The value to be set on the `--token-ttl` flag. |
| imagePullPolicy | string | `"IfNotPresent"` | The imagePullPolicy to be used on the operator. |
| imageVersion | string | `"latest"` | The image version used for the operator. |
| prometheusMonitoring.enable | bool | `true` | Create the `Service` and `ServiceMonitor` objects to enable Prometheus monitoring on the operator. |
| resources | object | `nil` | The resources requests/limits to be set on the deployment pod spec template. |
| serviceAccount.name | string | `"vault-dynamic-configuration-operator"` | The name of the `ServiceAccount` to be created. |
| watchNamespace | string | `""` | The value to be set on the `WATCH_NAMESPACE` environment variable. |
