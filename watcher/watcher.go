package watcher

import (
	"gitlab.daocloud.cn/mesh/ckube/store"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"time"
)

type watcher struct {
	client    kubernetes.Interface
	resources []store.GroupVersionResource
	store     store.Store
	stop      chan struct{}
	Watcher
}

func NewWatcher(client kubernetes.Interface, resources []store.GroupVersionResource, store store.Store) Watcher {
	return &watcher{
		client:    client,
		resources: resources,
		store:     store,
		stop:      make(chan struct{}),
	}
}

func (w *watcher) Stop() error {
	close(w.stop)
	return nil
}

func (w *watcher) Start() error {
	for _, r := range w.resources {
		go func(r store.GroupVersionResource) {
			inf, err := informers.NewSharedInformerFactory(w.client, time.Hour).ForResource(schema.GroupVersionResource{
				Group:    r.Group,
				Version:  r.Version,
				Resource: r.Resource,
			})
			if err != nil {
				return
			}
			inf.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					w.store.OnResourceAdded(r, obj)
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					w.store.OnResourceModified(r, newObj)
				},
				DeleteFunc: func(obj interface{}) {
					w.store.OnResourceDeleted(r, obj)
				},
			})
			inf.Informer().Run(w.stop)
		}(r)
	}
	return nil
}
