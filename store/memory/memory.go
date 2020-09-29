package memory

import (
	"bytes"
	"fmt"
	"gitlab.daocloud.cn/mesh/ckube/log"
	"gitlab.daocloud.cn/mesh/ckube/store"
	"k8s.io/client-go/util/jsonpath"
	"sort"
	"strings"
)

type resourceObj map[string]store.Object

type namespaceResource map[string]resourceObj

// todo add lock
type memoryStore struct {
	resourceMap map[store.GroupVersionResource]namespaceResource
	indexConf   map[store.GroupVersionResource]map[string]string
	store.Store
}

func NewMemoryStore(indexConf map[store.GroupVersionResource]map[string]string) store.Store {
	s := memoryStore{
		indexConf: indexConf,
	}
	resourceMap := make(map[store.GroupVersionResource]namespaceResource)
	for k, _ := range indexConf {
		resourceMap[k] = namespaceResource{}
	}
	s.resourceMap = resourceMap
	return &s
}

func (m *memoryStore) initResourceNamespace(gvr store.GroupVersionResource, namespace string) {
	if _, ok := m.resourceMap[gvr][namespace]; ok {
		return
	}
	m.resourceMap[gvr][namespace] = resourceObj{}
}

func (m *memoryStore) OnResourceAdded(gvr store.GroupVersionResource, obj interface{}) error {
	ns, name, o := m.buildResourceWithIndex(gvr, obj)
	m.initResourceNamespace(gvr, ns)
	m.resourceMap[gvr][ns][name] = o
	return nil
}

func (m *memoryStore) OnResourceModified(gvr store.GroupVersionResource, obj interface{}) error {
	ns, name, o := m.buildResourceWithIndex(gvr, obj)
	m.resourceMap[gvr][ns][name] = o
	return nil
}

func (m *memoryStore) OnResourceDeleted(gvr store.GroupVersionResource, obj interface{}) error {
	ns, name, _ := m.buildResourceWithIndex(gvr, obj)
	m.initResourceNamespace(gvr, ns)
	delete(m.resourceMap[gvr][ns], name)
	return nil
}

type Filter func(obj store.Object) (bool, error)

func searchToFilter(search string) Filter {
	// a=1
	// index  1
	//
	indexOfEqual := strings.Index(search, "=")
	if indexOfEqual < 0 {
		return nil
	}
	key := search[:indexOfEqual]
	value := ""
	if indexOfEqual < len(search)-1 {
		value = search[indexOfEqual+1:]
	}
	return func(obj store.Object) (bool, error) {
		if v, ok := obj.Index[key]; !ok {
			return false, fmt.Errorf("unexpected search key: %s", key)
		} else {
			return strings.Contains(v, value), nil
		}
	}
}

func (m *memoryStore) Query(gvr store.GroupVersionResource, query store.Query) store.QueryResult {
	res := store.QueryResult{}
	resources := make([]store.Object, 0)
	filter := searchToFilter(query.Search)
	for ns, robj := range m.resourceMap[gvr] {
		if query.Namespace == "" || query.Namespace == ns {
			for _, obj := range robj {
				if filter == nil {
					resources = append(resources, obj)
				} else {
					if ok, err := filter(obj); ok {
						resources = append(resources, obj)
					} else if err != nil {
						res.Error = err
					}
				}
			}
		}
	}
	l := int64(len(resources))
	if l == 0 {
		return res
	}
	if _, ok := resources[0].Index[query.Sort]; query.Sort != "" && !ok {
		res.Error = fmt.Errorf("unexpected sort key: %s", query.Sort)
	}
	if query.Sort != "" {
		sort.Slice(resources, func(i, j int) bool {
			r := resources[i].Index[query.Sort] < resources[j].Index[query.Sort]
			if query.Reverse {
				r = !r
			}
			return r
		})
	}
	res.Total = l
	var start, end int64 = 0, 0
	if query.PageSize == 0 {
		// all resources
		start = 0
		end = l
	} else {
		start = (query.Page - 1) * query.PageSize
		end = start + query.PageSize
		if start >= l-1 {
			start = l - 1
		}
		if end >= l {
			end = l
		}
	}
	for _, r := range resources[start:end] {
		res.Items = append(res.Items, r.Obj)
	}
	return res
}

func (m *memoryStore) buildResourceWithIndex(gvr store.GroupVersionResource, obj interface{}) (string, string, store.Object) {
	s := store.Object{
		Index: map[string]string{},
		Obj:   obj,
	}
	jp := jsonpath.New("parser")
	jp.AllowMissingKeys(true)
	for k, v := range m.indexConf[gvr] {
		w := bytes.NewBuffer([]byte{})
		jp.Parse(v)
		err := jp.Execute(w, obj)
		if err != nil {
			log.Warnf("exec jsonpath error: %v, %v", obj, err)
		}
		s.Index[k] = w.String()
	}
	namespace := ""
	name := ""
	if ns, ok := s.Index["namespace"]; ok {
		namespace = ns
	}
	if n, ok := s.Index["name"]; ok {
		name = n
	}
	return namespace, name, s
}
