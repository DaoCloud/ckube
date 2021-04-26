package memory

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"gitlab.daocloud.cn/mesh/ckube/common"
	"gitlab.daocloud.cn/mesh/ckube/log"
	"gitlab.daocloud.cn/mesh/ckube/store"
	"gitlab.daocloud.cn/mesh/ckube/utils"
	"k8s.io/client-go/util/jsonpath"
)

type resourceObj struct {
	lock   *sync.RWMutex
	objMap map[string]store.Object
}

type namespaceResource map[string]resourceObj

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
	m.resourceMap[gvr][namespace] = resourceObj{
		lock:   &sync.RWMutex{},
		objMap: map[string]store.Object{},
	}
}

func (m *memoryStore) IsStoreGVR(gvr store.GroupVersionResource) bool {
	_, ok := m.indexConf[gvr]
	return ok
}

func (m *memoryStore) Clean(gvr store.GroupVersionResource) error {
	if _, ok := m.resourceMap[gvr]; ok {
		m.resourceMap[gvr] = namespaceResource{}
		return nil
	}
	return fmt.Errorf("resource %s not found", gvr)
}

func (m *memoryStore) OnResourceAdded(gvr store.GroupVersionResource, obj interface{}) error {
	ns, name, o := m.buildResourceWithIndex(gvr, obj)
	m.initResourceNamespace(gvr, ns)
	m.resourceMap[gvr][ns].lock.Lock()
	defer m.resourceMap[gvr][ns].lock.Unlock()
	m.resourceMap[gvr][ns].objMap[name] = o
	return nil
}

func (m *memoryStore) OnResourceModified(gvr store.GroupVersionResource, obj interface{}) error {
	ns, name, o := m.buildResourceWithIndex(gvr, obj)
	m.resourceMap[gvr][ns].lock.Lock()
	defer m.resourceMap[gvr][ns].lock.Unlock()
	m.resourceMap[gvr][ns].objMap[name] = o
	return nil
}

func (m *memoryStore) OnResourceDeleted(gvr store.GroupVersionResource, obj interface{}) error {
	ns, name, _ := m.buildResourceWithIndex(gvr, obj)
	m.initResourceNamespace(gvr, ns)
	m.resourceMap[gvr][ns].lock.Lock()
	defer m.resourceMap[gvr][ns].lock.Unlock()
	delete(m.resourceMap[gvr][ns].objMap, name)
	return nil
}

type innerSort struct {
	key     string
	typ     string
	reverse bool
}

func sortObjs(objs []store.Object, s string) ([]store.Object, error) {
	if s == "" {
		s = "namespace, name"
	}
	if len(objs) == 0 {
		return objs, nil
	}
	checkKeyMap := objs[0].Index
	ss := strings.Split(s, ",")
	sorts := make([]innerSort, 0, len(ss))
	for _, s = range ss {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		st := innerSort{
			reverse: false,
			typ:     common.KeyTypeStr,
		}
		if strings.Contains(s, " ") {
			parts := strings.Split(s, " ")
			if len(parts) > 2 {
				return objs, nil
			}
			if len(parts) == 2 {
				switch parts[1] {
				case common.SortDesc:
					st.reverse = true
				case common.SortASC:
					st.reverse = false
				default:
					return objs, fmt.Errorf("error sort format `%s`", parts[1])
				}
			}
			// override s
			s = parts[0]
		}
		if strings.Contains(s, common.KeyTypeSep) {
			parts := strings.Split(s, common.KeyTypeSep)
			if len(parts) != 2 {
				return objs, fmt.Errorf("error type format")
			}
			switch parts[1] {
			case common.KeyTypeInt:
				st.typ = common.KeyTypeInt
			case common.KeyTypeStr:
				st.typ = common.KeyTypeStr
			default:
				return objs, fmt.Errorf("unsupported typ: %s", parts[1])
			}
			s = parts[0]
		}
		st.key = s
		if _, ok := checkKeyMap[s]; !ok {
			return objs, fmt.Errorf("unexpected sort key: %s", s)
		}
		sorts = append(sorts, st)
	}
	var sortErr error = nil
	sort.Slice(objs, func(i, j int) bool {
		for _, s := range sorts {
			r := false
			equals := false
			vis := objs[i].Index[s.key]
			vjs := objs[j].Index[s.key]
			if s.typ == common.KeyTypeInt {
				keyErr := fmt.Errorf("value of `%s` can not convert to number", s.key)
				vi, err := strconv.ParseFloat(vis, 64)
				if err != nil {
					sortErr = keyErr
					break
				}
				vj, err := strconv.ParseFloat(vjs, 64)
				if err != nil {
					sortErr = keyErr
					break
				}
				r = vi < vj
				equals = vi == vj
			} else {
				r = vis < vjs
				equals = vis == vjs
			}
			if equals {
				continue
			}
			if s.reverse {
				r = !r
			}
			return r
		}
		return true
	})
	return objs, sortErr
}

func (m *memoryStore) Get(gvr store.GroupVersionResource, namespace, name string) interface{} {
	if m.resourceMap[gvr] != nil {
		if nsObjs, ok := m.resourceMap[gvr][namespace]; ok {
			nsObjs.lock.RLock()
			defer nsObjs.lock.RUnlock()
			if sobj, ok := nsObjs.objMap[name]; ok {
				return sobj.Obj
			}
		}
	}
	return nil
}

func (m *memoryStore) Query(gvr store.GroupVersionResource, query store.Query) store.QueryResult {
	res := store.QueryResult{}
	resources := make([]store.Object, 0)
	for ns, robj := range m.resourceMap[gvr] {
		if query.Namespace == "" || query.Namespace == ns {
			robj.lock.RLock()
			for _, obj := range robj.objMap {
				if ok, err := query.Match(obj.Index); ok {
					resources = append(resources, obj)
				} else if err != nil {
					res.Error = err
				}
			}
			robj.lock.RUnlock()
		}
	}
	l := int64(len(resources))
	if l == 0 {
		return res
	}
	resources, err := sortObjs(resources, query.Sort)
	if err != nil {
		res.Error = err
		return res
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
		if start >= l {
			start = l
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
	mobj := utils.Obj2JSONMap(obj)
	for k, v := range m.indexConf[gvr] {
		w := bytes.NewBuffer([]byte{})
		jp.Parse(v)
		err := jp.Execute(w, mobj)
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
