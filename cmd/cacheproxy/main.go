package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"gitlab.daocloud.cn/dsm-public/common/log"
	"gitlab.daocloud.cn/mesh/ckube/common"
	"gitlab.daocloud.cn/mesh/ckube/server"
	"gitlab.daocloud.cn/mesh/ckube/store"
	"gitlab.daocloud.cn/mesh/ckube/store/memory"
	"gitlab.daocloud.cn/mesh/ckube/utils/prommonitor"
	"gitlab.daocloud.cn/mesh/ckube/watcher"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
)

func GetK8sConfigConfigWithFile(kubeconfig, context string) *rest.Config {
	config, _ := rest.InClusterConfig()
	if config != nil {
		return config
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

	config, _ = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides).ClientConfig()
	return config
}

func GetKubernetesClientWithFile(kubeconfig, context string) (kubernetes.Interface, error) {
	clientset, err := kubernetes.NewForConfig(GetK8sConfigConfigWithFile(kubeconfig, context))
	if err != nil {
		panic(err.Error())
	}
	return clientset, err
}

func main() {
	configFile := ""
	listen := ":3033"
	debug := false
	flag.StringVar(&configFile, "c", "config/local.json", "config file path")
	flag.StringVar(&listen, "a", ":3033", "listen port")
	flag.BoolVar(&debug, "d", false, "debug mode")
	flag.Parse()
	if debug {
		log.SetDebug()
	}

	cfg := common.Config{}
	if bs, err := ioutil.ReadFile(configFile); err != nil {
		fmt.Fprintf(os.Stderr, "config file load error: %v", err)
		os.Exit(1)
	} else {
		err := json.Unmarshal(bs, &cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "config file load error: %v", err)
			os.Exit(3)
		}
	}
	common.InitConfig(&cfg)
	client, err := GetKubernetesClientWithFile("", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "init k8s client error: %v", err)
		os.Exit(2)
	}

	// 记录组件运行状态
	prommonitor.Up.WithLabelValues(prommonitor.CkubeComponent).Set(1)

	indexConf := map[store.GroupVersionResource]map[string]string{}
	storeGVRConfig := []store.GroupVersionResource{}
	for _, proxy := range cfg.Proxies {
		indexConf[store.GroupVersionResource{
			Group:    proxy.Group,
			Version:  proxy.Version,
			Resource: proxy.Resource,
		}] = proxy.Index
		storeGVRConfig = append(storeGVRConfig, store.GroupVersionResource{
			Group:    proxy.Group,
			Version:  proxy.Version,
			Resource: proxy.Resource,
		})
	}
	m := memory.NewMemoryStore(indexConf)
	w := watcher.NewWatcher(*GetK8sConfigConfigWithFile("", ""), client, storeGVRConfig, m)
	w.Start()
	ser := server.NewMuxServer(listen, client, m)
	ser.Run()
}
