#!/bin/bash

helm package helm/vault-dynamic-configuration-operator/
version=$(cat helm/vault-dynamic-configuration-operator/Chart.yaml | yaml2json | jq -r '.version')
mv vault-dynamic-configuration-operator-$version.tgz docs/
helm repo index docs --url https://patoarvizu.github.io/vault-dynamic-configuration-operator
helm-docs
mv helm/vault-dynamic-configuration-operator/README.md docs/index.md
git add docs/