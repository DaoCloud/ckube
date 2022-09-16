package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kubeapi "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/yaml"

	"github.com/DaoCloud/ckube/common"
	"github.com/DaoCloud/ckube/log"
	"github.com/DaoCloud/ckube/server"
	"github.com/DaoCloud/ckube/store"
	"github.com/DaoCloud/ckube/store/memory"
	"github.com/DaoCloud/ckube/utils"
	"github.com/DaoCloud/ckube/utils/prommonitor"
	"github.com/DaoCloud/ckube/watcher"
)

func GetK8sConfigConfigWithFile(kubeconfig, context string) *rest.Config {
	var config *rest.Config
	if kubeconfig == "" && context == "" {
		config, _ := rest.InClusterConfig()
		if config != nil {
			return config
		}
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
		return nil, err
	}
	return clientset, err
}

func loadFromConfig(kubeConfig, configFile string) (map[string]kubernetes.Interface, watcher.Watcher, store.Store, error) {

	cfg := common.Config{}
	if bs, err := os.ReadFile(configFile); err != nil {
		log.Errorf("config file load error: %v", err)
		return nil, nil, nil, err
	} else {
		err := json.Unmarshal(bs, &cfg)
		if err != nil {
			log.Errorf("config file load error: %v", err)
			return nil, nil, nil, err
		}
	}
	clusterConfigs := map[string]rest.Config{}
	clusterClients := map[string]kubernetes.Interface{}
	kubecfg := kubeapi.Config{}
	if kubeConfig == "" {
		defaultConfig := path.Join(os.Getenv("HOME"), ".kube/config")
		if _, err := os.Stat(defaultConfig); err != nil {
			// may running in pod
			log.Info("no kube config found, load from service account.")
			c := GetK8sConfigConfigWithFile(kubeConfig, "")
			if c == nil {
				log.Errorf("init k8s config from service account error")
				return nil, nil, nil, fmt.Errorf("init k8s config error")
			}
			if cfg.DefaultCluster == "" {
				cfg.DefaultCluster = "default"
			}
			clusterConfigs[cfg.DefaultCluster] = *c
			client, err := GetKubernetesClientWithFile(kubeConfig, "")
			if err != nil {
				return nil, nil, nil, err
			}
			clusterClients[cfg.DefaultCluster] = client
		} else {
			kubeConfig = defaultConfig
		}
	}
	if kubeConfig != "" {
		bs, err := os.ReadFile(kubeConfig)
		if err != nil {
			log.Errorf("read kube config error: %v", err)
			return nil, nil, nil, err
		}
		err = yaml.Unmarshal(bs, &kubecfg)
		if err != nil {
			err = json.Unmarshal(bs, &kubecfg)
			if err != nil {
				log.Errorf("parse kube config %s error: %v", kubeConfig, err)
				return nil, nil, nil, err
			}
		}
		log.Debugf("got kube config: %s", bs)
		cfg.DefaultCluster = kubecfg.CurrentContext

		for _, ctx := range kubecfg.Contexts {
			c := GetK8sConfigConfigWithFile(kubeConfig, ctx.Name)
			if c == nil {
				log.Errorf("init k8s config error")
				return nil, nil, nil, fmt.Errorf("init k8s config error")
			}
			clusterConfigs[ctx.Name] = *c
			client, err := GetKubernetesClientWithFile(kubeConfig, ctx.Name)
			if err != nil {
				log.Errorf("init k8s client error: %v", err)
				return nil, nil, nil, err
			}
			clusterClients[ctx.Name] = client
		}
	}
	common.InitConfig(&cfg)

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
	w := watcher.NewWatcher(clusterConfigs, storeGVRConfig, m)
	_ = w.Start()
	return clusterClients, w, m, nil
}

func main() {
	configFile := ""
	listen := ":80"
	kubeConfig := ""
	debug := false
	defaultConfig := path.Join(os.Getenv("HOME"), ".kube/config")
	flag.StringVar(&configFile, "c", "config/local.json", "config file path")
	flag.StringVar(&listen, "a", ":80", "listen port")
	flag.StringVar(&kubeConfig, "k", "", "kube config file name")
	flag.BoolVar(&debug, "d", false, "debug mode")
	flag.Parse()
	if debug {
		log.SetDebug()
	}
	clis, w, s, err := loadFromConfig(kubeConfig, configFile)
	if err != nil {
		log.Errorf("load from config file error: %v", err)
		os.Exit(1)
	}
	ser := server.NewMuxServer(listen, clis, s)
	files := []string{configFile}
	if kubeConfig == "" {
		files = append(files, defaultConfig)
	} else {
		files = append(files, kubeConfig)
	}
	fixedWatcher, err := utils.NewFixedFileWatcher(files)
	if err != nil {
		log.Errorf("create watcher error: %v", err)
	} else {
		if err := fixedWatcher.Start(); err != nil {
			panic(fmt.Errorf("watcher start error: %v", err))
		}
		defer fixedWatcher.Close()
		go func() {
			for e := range fixedWatcher.Events() {
				log.Infof("get file watcher event: %v", e)
				switch e.Type {
				case utils.EventTypeChanged:
					// do reload
				case utils.EventTypeError:
					log.Errorf("got file watcher error type: file: %s", e.Name)
					// do reload
				}
				clis, rw, rs, err := loadFromConfig(kubeConfig, configFile)
				if err != nil {
					prommonitor.ConfigReload.WithLabelValues("failed").Inc()
					log.Errorf("watcher: reload config error: %v", err)
					continue
				}
				prommonitor.Resources.Reset()
				_ = w.Stop()
				w = rw
				ser.ResetStore(rs, clis) // reset store
				prommonitor.ConfigReload.WithLabelValues("success").Inc()
				log.Infof("auto reloaded config successfully")
			}
		}()
	}
	_ = ser.Run()
}
