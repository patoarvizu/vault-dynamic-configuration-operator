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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	sidecarInjectionMode        = "sidecar"
	initContainerInjectionMode  = "init-container"
	agentAutoInjectAnnotation   = "agent-auto-inject"
	configMapOverrideAnnotation = "agent-config-map"
	vaultAgentVolumeMountName   = "vault-agent"
	vaultAgentVolumeMountPath   = "/vault-agent"
)

type webhookCfg struct {
	certFile             string
	keyFile              string
	addr                 string
	annotationPrefix     string
	targetVaultAddress   string
	kubernetesAuthPath   string
	vaultImageVersion    string
	defaultConfigMapName string
	cpuRequest           string
	cpuLimit             string
	memoryRequest        string
	memoryLimit          string
}

var cfg = &webhookCfg{}
var injectionMode string

func injectVaultSidecar(_ context.Context, obj metav1.Object) (bool, error) {
	logger := &log.Std{}
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return false, nil
	}

	if pod.Annotations == nil || len(pod.Annotations) == 0 {
		return false, nil
	}

	if pod.Annotations["vault-sidecar-injected"] == "true" {
		return false, nil
	}

	if val, ok := pod.Annotations[fmt.Sprintf("%s/%s", cfg.annotationPrefix, agentAutoInjectAnnotation)]; !ok && val != sidecarInjectionMode && val != initContainerInjectionMode {
		return false, nil
	} else {
		injectionMode = val
	}

	configMapName := cfg.defaultConfigMapName
	if val, ok := pod.Annotations[fmt.Sprintf("%s/%s", cfg.annotationPrefix, configMapOverrideAnnotation)]; ok && val != "" {
		configMapName = val
	}

	logger.Infof("Injecting Vault sidecar into pod with service account %s", pod.Spec.ServiceAccountName)
	for i, c := range pod.Spec.Containers {
		if injectionMode == initContainerInjectionMode {
			pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts, corev1.VolumeMount{
				Name:      vaultAgentVolumeMountName,
				MountPath: vaultAgentVolumeMountPath,
			})
		} else {
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
						Name: configMapName,
					},
				},
			},
		},
	)

	if injectionMode == initContainerInjectionMode {
		pod.Spec.Volumes = append(pod.Spec.Volumes,
			corev1.Volume{
				Name: vaultAgentVolumeMountName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			},
		)
	}

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

	if injectionMode == sidecarInjectionMode {
		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
			Name:  "vault-agent",
			Image: "vault:" + cfg.vaultImageVersion,
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
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(cfg.cpuLimit),
					corev1.ResourceMemory: resource.MustParse(cfg.memoryLimit),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(cfg.cpuRequest),
					corev1.ResourceMemory: resource.MustParse(cfg.memoryRequest),
				},
			},
		})
	} else if injectionMode == initContainerInjectionMode {
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:  "vault-agent",
			Image: "vault:" + cfg.vaultImageVersion,
			Args: []string{
				"agent",
				"-config=/etc/vault/vault-agent-config.hcl",
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "vault-config",
					MountPath: "/etc/vault",
				},
				{
					Name:      vaultAgentVolumeMountName,
					MountPath: vaultAgentVolumeMountPath,
				},
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(cfg.cpuLimit),
					corev1.ResourceMemory: resource.MustParse(cfg.memoryLimit),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(cfg.cpuRequest),
					corev1.ResourceMemory: resource.MustParse(cfg.memoryRequest),
				},
			},
		})
	}

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
	fl.StringVar(&cfg.targetVaultAddress, "target-vault-address", "https://vault:8200", "Address of remote Vault API")
	fl.StringVar(&cfg.kubernetesAuthPath, "kubernetes-auth-path", "auth/kubernetes", "Path to Vault Kubernetes auth endpoint")
	fl.StringVar(&cfg.vaultImageVersion, "vault-image-version", "1.3.0", "Tag on the 'vault' Docker image to inject with the sidecar")
	fl.StringVar(&cfg.defaultConfigMapName, "default-config-map-name", "vault-agent-config", "The name of the ConfigMap to be used for the Vault agent configuration by default, unless overwritten by annotation")
	fl.StringVar(&cfg.cpuRequest, "cpu-request", "50m", "The amount of CPU units to request for the Vault agent sidecar")
	fl.StringVar(&cfg.cpuLimit, "cpu-limit", "100m", "The amount of CPU units to limit to on the Vault agent sidecar")
	fl.StringVar(&cfg.memoryRequest, "memory-request", "128Mi", "The amount of memory units to request for the Vault agent sidecar")
	fl.StringVar(&cfg.memoryLimit, "memory-limit", "256Mi", "The amount of memory units to limit to on the Vault agent sidecar")
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
