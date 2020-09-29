package main

import (
	"fmt"
	"gitlab.daocloud.cn/mesh/ckube/server"
	"gitlab.daocloud.cn/mesh/ckube/store"
	"gitlab.daocloud.cn/mesh/ckube/store/memory"
	"gitlab.daocloud.cn/mesh/ckube/watcher"
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
	client, err := GetKubernetesClientWithFile("", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "init k8s client error: %v", err)
		os.Exit(1)
	}
	podGvr := store.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}
	svcGvr := store.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}
	commonMap := map[string]string{
		"namespace":  "{.metadata.namespace}",
		"name":       "{.metadata.name}",
		"created_at": "{.metadata.creationTimestamp}",
	}
	m := memory.NewMemoryStore(map[store.GroupVersionResource]map[string]string{
		podGvr: {
			"namespace":                     "{.metadata.namespace}",
			"name":                          "{.metadata.name}",
			"creationTimestamp":             "{.metadata.creationTimestamp}",
			"metadata.ownerReferences.kind": "{.metadata.ownerReferences[0].kind}",
			"metadata.ownerReferences.name": "{.metadata.ownerReferences[0].name}",
		},
		svcGvr: commonMap,
	})
	w := watcher.NewWatcher(client, []store.GroupVersionResource{podGvr, svcGvr}, m)
	w.Start()
	ser := server.NewMuxServer(":3033", m)
	ser.Run()
	//podList, err := client.CoreV1().Pods("default").List(context.Background(), v1.ListOptions{})
	//for _, pod := range podList.Items {
	//	err := m.OnResourceAdded(podGvr, pod)
	//	fmt.Sprintf("err: %v", err)
	//}
	//time.Sleep(time.Second * 3)
	//res := m.Query(podGvr, store.Query{
	//	Namespace: "",
	//	Paginate: store.Paginate{
	//		Page:     3,
	//		PageSize: 1,
	//		Reverse:  true,
	//		Sort:     "name",
	//		Search:   "",
	//	},
	//})
	//fmt.Sprintf("%v", res)
	//make(chan struct{}) <- struct{}{}
}
