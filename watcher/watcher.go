package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"gitlab.daocloud.cn/mesh/ckube/common"
	"gitlab.daocloud.cn/mesh/ckube/log"
	"gitlab.daocloud.cn/mesh/ckube/store"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"strings"
	"sync"
	"time"
)

type watcher struct {
	config    rest.Config
	client    kubernetes.Interface
	resources []store.GroupVersionResource
	store     store.Store
	stop      chan struct{}
	lock      sync.Mutex
	Watcher
}

func NewWatcher(config rest.Config, client kubernetes.Interface, resources []store.GroupVersionResource, store store.Store) Watcher {
	return &watcher{
		config:    config,
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

type ObjType struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Data          map[string]interface{}
}

func (o *ObjType) UnmarshalJSON(bytes []byte) error {
	m := map[string]interface{}{}
	json.Unmarshal(bytes, &m)
	if v, ok := m["apiVersion"]; ok {
		o.APIVersion = v.(string)
	}
	if v, ok := m["kind"]; ok {
		o.Kind = v.(string)
	}
	if meta, ok := m["metadata"]; ok {
		bs, _ := json.Marshal(meta)
		json.Unmarshal(bs, &o.ObjectMeta)
	}
	delete(m, "apiVersion")
	delete(m, "kind")
	delete(m, "metadata")
	o.Data = m
	return nil
}

func (o *ObjType) MarshalJSON() ([]byte, error) {
	bsm, _ := json.Marshal(o.Data)
	bso, _ := json.Marshal(struct {
		v1.TypeMeta   `json:",inline"`
		v1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	}{
		TypeMeta:   o.TypeMeta,
		ObjectMeta: o.ObjectMeta,
	})
	if string(bsm) == "{}" {
		return bso, nil
	}
	if string(bso) == "{}" {
		return bsm, nil
	}
	bsm = bsm[:len(bsm)-1]
	bso = bso[1:]
	bs := make([]byte, 0, len(bsm)+len(bso)+1)
	bs = append(bs, bsm...)
	bs = append(bs, ',')
	bs = append(bs, bso...)
	return bs, nil
}

func (o ObjType) GetObjectKind() schema.ObjectKind {
	return &o
}

func (o ObjType) DeepCopyObject() runtime.Object {
	//o.lock.Lock()
	//defer o.lock.Unlock()
	m := map[string]interface{}{}
	for k, v := range o.Data {
		m[k] = v
	}
	return &ObjType{
		TypeMeta:   o.TypeMeta,
		ObjectMeta: o.ObjectMeta,
		Data:       m,
	}
}

func (w *watcher) Start() error {
	for _, r := range w.resources {
		go func(r store.GroupVersionResource) {
			gvk := schema.GroupVersionKind{
				Group:   r.Group,
				Version: r.Version,
				Kind:    strings.TrimRight(common.GetGVRKind(r.Group, r.Version, r.Resource), "List"),
			}
			gv := schema.GroupVersion{
				Group:   r.Group,
				Version: r.Version,
			}
			w.lock.Lock()
			if _, ok := scheme.Scheme.KnownTypes(gv)[gvk.Kind]; !ok {
				scheme.Scheme.AddKnownTypeWithName(gvk, &ObjType{})
			}
			w.lock.Unlock()

			w.config.GroupVersion = &schema.GroupVersion{
				Group:   r.Group,
				Version: r.Version,
			}
			scheme.Codecs.UniversalDeserializer()
			w.config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
			rt, _ := rest.RESTClientFor(&w.config)
			for {
				ctx, calcel := context.WithTimeout(context.Background(), time.Hour)
				url := ""
				if r.Group == "" {
					url = fmt.Sprintf("/api/%s/%s?watch=true", r.Version, r.Resource)
				} else {
					url = fmt.Sprintf("/apis/%s/%s/%s?watch=true", r.Group, r.Version, r.Resource)
				}
				ww, err := rt.Get().RequestURI(url).Timeout(time.Hour).Watch(ctx)
				if err != nil {
					log.Errorf("create watcher error: %v", err)
					time.Sleep(time.Second * 3)
				} else {
				resultChan:
					for {
						select {
						case rr, open := <-ww.ResultChan():
							if open {
								switch rr.Type {
								case watch.Added:
									w.store.OnResourceAdded(r, rr.Object)
								case watch.Modified:
									w.store.OnResourceModified(r, rr.Object)
								case watch.Deleted:
									w.store.OnResourceDeleted(r, rr.Object)
								case watch.Error:
									log.Warnf("watch stream(%v) error: %v", r, rr.Object)
								}
							} else {
								w.store.Clean(r)
								log.Warnf("watch stream(%v) closed", r)
								break resultChan
							}
						case <-w.stop:
							break resultChan
						}
					}
				}
				calcel()
			}
		}(r)
	}
	return nil
}
