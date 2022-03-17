package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/DaoCloud/ckube/common"
	"github.com/DaoCloud/ckube/log"
	"github.com/DaoCloud/ckube/store"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type watcher struct {
	clusterConfigs map[string]rest.Config
	resources      []store.GroupVersionResource
	store          store.Store
	stop           chan struct{}
	lock           sync.Mutex
	Watcher
}

func NewWatcher(clusterConfigs map[string]rest.Config, resources []store.GroupVersionResource, store store.Store) Watcher {
	return &watcher{
		clusterConfigs: clusterConfigs,
		resources:      resources,
		store:          store,
		stop:           make(chan struct{}),
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

func (w *watcher) watchResources(r store.GroupVersionResource, cluster string) {
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
	config := w.clusterConfigs[cluster]

	config.GroupVersion = &schema.GroupVersion{
		Group:   r.Group,
		Version: r.Version,
	}
	scheme.Codecs.UniversalDeserializer()
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	rt, _ := rest.RESTClientFor(&config)
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
			log.Errorf("cluster(%s): create watcher for %s error: %v", cluster, url, err)
			time.Sleep(time.Second * 3)
		} else {
		resultChan:
			for {
				select {
				case rr, open := <-ww.ResultChan():
					if open {
						switch rr.Type {
						case watch.Added:
							w.store.OnResourceAdded(r, cluster, rr.Object)
						case watch.Modified:
							w.store.OnResourceModified(r, cluster, rr.Object)
						case watch.Deleted:
							w.store.OnResourceDeleted(r, cluster, rr.Object)
						case watch.Error:
							log.Warnf("cluster(%s): watch stream(%v) error: %v", cluster, r, rr.Object)
						}
					} else {
						w.store.Clean(r, cluster)
						log.Warnf("cluster(%s): watch stream(%v) closed", cluster, r)
						break resultChan
					}
				case <-w.stop:
					break resultChan
				}
			}
		}
		calcel()
	}
}

func (w *watcher) Start() error {
	for _, r := range w.resources {
		for c := range w.clusterConfigs {
			go w.watchResources(r, c)
		}
	}
	return nil
}
