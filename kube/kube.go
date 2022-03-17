package kube

import (
	"os"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetK8sConfigConfigWithFile(kubeconfig, context string) (*rest.Config, error) {
	config, _ := rest.InClusterConfig()
	if config != nil {
		return config, nil
	}
	if kubeconfig != "" {
		info, err := os.Stat(kubeconfig)
		if err != nil || info.Size() == 0 {
			// If the specified kubeconfig doesn't exists / empty file / any other error
			// from file stat, fall back to default
			kubeconfig = ""
		}
	}

	// Config loading rules:
	// 1. kubeconfig if it not empty string
	// 2. In cluster config if running in-cluster
	// 3. Config(s) in KUBECONFIG environment variable
	// 4. Use $HOME/.kube/config
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	loadingRules.ExplicitPath = kubeconfig
	configOverrides := &clientcmd.ConfigOverrides{
		ClusterDefaults: clientcmd.ClusterDefaults,
		CurrentContext:  context,
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides).ClientConfig()

	return config, err
}
