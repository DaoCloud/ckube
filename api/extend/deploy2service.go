package extend

import (
	v1 "k8s.io/api/core/v1"
	"strings"
	"sync"

	"gitlab.daocloud.cn/mesh/ckube/utils"
	//appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"time"
)

var podsMap = make(map[string]map[string]*v1.Pod)
var podMutex = sync.RWMutex{}

func getPodDeploymentName(pod *v1.Pod) string {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "ReplicaSet" {
			return strings.Join(strings.Split(ref.Name, "-")[:strings.Count(ref.Name, "-")], "-")
		}
	}
	return ""
}

func isServicesPod(svc *v1.Service, pod *v1.Pod) bool {
	return utils.IsSubsetOf(svc.Spec.Selector, pod.Labels)
}

func initPodInformer(client kubernetes.Interface, stop <-chan struct{}) error {
	inf := informers.NewSharedInformerFactory(client, time.Hour).Core().V1().Pods().Informer()
	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod, ok := obj.(*v1.Pod)
			if !ok {
				return
			}
			deploy := getPodDeploymentName(pod)
			podMutex.Lock()
			defer podMutex.Unlock()
			depKey := pod.Namespace + "/" + deploy
			if _, ok := podsMap[depKey]; !ok {
				podsMap[depKey] = map[string]*v1.Pod{}
			}
			podsMap[depKey][pod.Name] = pod
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pod, ok := newObj.(*v1.Pod)
			if !ok {
				return
			}
			deploy := getPodDeploymentName(pod)
			podMutex.Lock()
			defer podMutex.Unlock()
			depKey := pod.Namespace + "/" + deploy
			podsMap[depKey][pod.Name] = pod
		},
		DeleteFunc: func(obj interface{}) {
			pod, ok := obj.(*v1.Pod)
			if !ok {
				return
			}
			deploy := getPodDeploymentName(pod)
			podMutex.Lock()
			defer podMutex.Unlock()
			depKey := pod.Namespace + "/" + deploy
			delete(podsMap[depKey], pod.Name)
			if len(podsMap[depKey]) == 0 {
				delete(podsMap, depKey)
			}
		},
	})
	go inf.Run(stop)
	return nil
}

func initServiceInformer(client kubernetes.Interface, stop <-chan struct{}) error {
	inf := informers.NewSharedInformerFactory(client, time.Hour).Core().V1().Services().Informer()
	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    nil,
		UpdateFunc: nil,
		DeleteFunc: nil,
	})
	go inf.Run(stop)
	return nil
}
