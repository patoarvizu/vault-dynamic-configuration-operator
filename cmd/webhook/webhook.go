package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	whhttp "github.com/slok/kubewebhook/pkg/http"
	"github.com/slok/kubewebhook/pkg/log"
	mutatingwh "github.com/slok/kubewebhook/pkg/webhook/mutating"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type webhookCfg struct {
	certFile             string
	keyFile              string
	addr                 string
	annotationPrefix     string
	autoInjectAnnotation string
	targetVaultAddress   string
	kubernetesAuthPath   string
}

var cfg = &webhookCfg{}

func injectVaultSidecar(_ context.Context, obj metav1.Object) (bool, error) {
	logger := &log.Std{}
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return false, nil
	}

	if pod.Annotations == nil || len(pod.Annotations) == 0 {
		return false, nil
	}

	if val, ok := pod.Annotations[fmt.Sprintf("%s/%s", cfg.annotationPrefix, cfg.autoInjectAnnotation)]; !ok || val != "true" {
		return false, nil
	}
	logger.Infof("Injecting Vault sidecar")
	for i, c := range pod.Spec.Containers {
		found := false
		for _, e := range c.Env {
			if e.Name == "VAULT_ADDR" {
				e.Value = "http://localhost:8200"
				found = true
			}
		}
		if !found {
			pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env, corev1.EnvVar{Name: "VAULT_ADDR", Value: "http://127.0.0.1:8200"})
		}
	}

	pod.Spec.Volumes = append(pod.Spec.Volumes,
		corev1.Volume{
			Name: "vault-config",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		},
		corev1.Volume{
			Name: "vault-config-template",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "vault-agent-config",
					},
				},
			},
		},
	)

	pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
		Name:  "config-template",
		Image: "hairyhenderson/gomplate:v3",
		Command: []string{
			"/gomplate",
			"--file",
			"/etc/template/vault-agent-config.hcl",
			"--out",
			"/etc/vault/vault-agent-config.hcl",
		},
		Env: []corev1.EnvVar{
			{
				Name:  "SERVICE",
				Value: pod.Spec.ServiceAccountName,
			},
			{
				Name:  "TARGET_VAULT_ADDRESS",
				Value: cfg.targetVaultAddress,
			},
			{
				Name:  "KUBERNETES_AUTH_PATH",
				Value: cfg.kubernetesAuthPath,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "vault-config",
				MountPath: "/etc/vault",
			},
			{
				Name:      "vault-config-template",
				MountPath: "/etc/template",
			},
		},
	})

	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
		Name:  "vault-agent",
		Image: "vault:1.3.0",
		Args: []string{
			"agent",
			"-config=/etc/vault/vault-agent-config.hcl",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "vault-config",
				MountPath: "/etc/vault",
			},
		},
	})

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	pod.Annotations["vault-sidecar-injected"] = "true"

	return false, nil
}

func main() {
	logger := &log.Std{}
	logger.Infof("Starting webhook!")

	fl := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fl.StringVar(&cfg.certFile, "tls-cert-file", "", "TLS certificate file")
	fl.StringVar(&cfg.keyFile, "tls-key-file", "", "TLS key file")
	fl.StringVar(&cfg.annotationPrefix, "annotation-prefix", "vault.patoarvizu.dev", "Prefix of the annotations the webhook will process")
	fl.StringVar(&cfg.autoInjectAnnotation, "agent-auto-inject-annotation", "agent-auto-inject", "Annotation the webhook will look for in pods")
	fl.StringVar(&cfg.targetVaultAddress, "target-vault-address", "https://vault:8200", "Address of remote Vault API")
	fl.StringVar(&cfg.kubernetesAuthPath, "kubernetes-auth-path", "auth/kubernetes", "Path to Vault Kubernetes auth endpoint")
	fl.StringVar(&cfg.addr, "listen-addr", ":4443", "The address to start the server")

	fl.Parse(os.Args[1:])

	pm := mutatingwh.MutatorFunc(injectVaultSidecar)

	mcfg := mutatingwh.WebhookConfig{
		Name: "vaultSidecarInjector",
		Obj:  &corev1.Pod{},
	}
	wh, err := mutatingwh.NewWebhook(mcfg, pm, nil, nil, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating webhook: %s", err)
		os.Exit(1)
	}
	whHandler, err := whhttp.HandlerFor(wh)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating webhook handler: %s", err)
		os.Exit(1)
	}
	err = http.ListenAndServeTLS(cfg.addr, cfg.certFile, cfg.keyFile, whHandler)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error serving webhook: %s", err)
		os.Exit(1)
	}
}
