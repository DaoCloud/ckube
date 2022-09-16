package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/samber/lo"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/jsonpath"

	"github.com/DaoCloud/ckube/common/constants"
	"github.com/DaoCloud/ckube/log"
	"github.com/DaoCloud/ckube/store"
	"github.com/DaoCloud/ckube/utils"
	"github.com/DaoCloud/ckube/utils/prommonitor"
)

type syncResourceStore[K comparable, V any] struct {
	lock      sync.RWMutex
	resources map[K]*V
}

func (s *syncResourceStore[K, V]) Set(key K, value V) {
	s.lock.Lock()
	if s.resources == nil {
		s.resources = make(map[K]*V)
	}
	s.resources[key] = &value
	s.lock.Unlock()
}

func (s *syncResourceStore[K, V]) Init(key K) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.resources == nil {
		s.resources = make(map[K]*V)
	}
	if _, ok := s.resources[key]; ok {
		return
	}
	s.resources[key] = new(V)
}

func (s *syncResourceStore[K, V]) Clean() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.resources = make(map[K]*V)
}

func (s *syncResourceStore[K, V]) Get(key K) *V {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.resources[key]
}

func (s *syncResourceStore[K, V]) Exists(key K) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	_, ok := s.resources[key]
	return ok
}

func (s *syncResourceStore[K, V]) Delete(key K) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.resources, key)
}

func (s *syncResourceStore[K, V]) Values() []*V {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return lo.Values(s.resources)
}

func (s *syncResourceStore[K, V]) ForEach(iter func(k K, v *V)) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	for k, v := range s.resources {
		iter(k, v)
	}
}

func (s *syncResourceStore[K, V]) Len() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return len(s.resources)
}

type clusterName string

type namespaceName string

type memoryStore struct {
	indexConf map[store.GroupVersionResource]map[string]string
	// resourceMap gvr - clusterName - namespaceName - name
	resourceMap syncResourceStore[
		store.GroupVersionResource,
		syncResourceStore[
			clusterName,
			syncResourceStore[
				namespaceName,
				syncResourceStore[string, store.Object],
			],
		],
	]
	store.Store
}

func NewMemoryStore(indexConf map[store.GroupVersionResource]map[string]string) store.Store {
	s := memoryStore{
		indexConf: indexConf,
	}
	for k := range indexConf {
		s.resourceMap.Init(k)
	}
	return &s
}

func (m *memoryStore) initResourceNamespace(gvr store.GroupVersionResource, cluster, namespace string) {
	m.resourceMap.Get(gvr).Init(clusterName(cluster))
	m.resourceMap.Get(gvr).Get(clusterName(cluster)).Init(namespaceName(namespace))
}

func (m *memoryStore) IsStoreGVR(gvr store.GroupVersionResource) bool {
	return m.resourceMap.Exists(gvr)
}

func (m *memoryStore) Clean(gvr store.GroupVersionResource, cluster string) error {
	if !m.resourceMap.Get(gvr).Exists(clusterName(cluster)) {
		return fmt.Errorf("cluster %s not exists", cluster)
	}
	for _, c := range m.resourceMap.Get(gvr).Get(clusterName(cluster)).Values() {
		c.Clean()
	}
	return nil
}

func (m *memoryStore) OnResourceAdded(gvr store.GroupVersionResource, cluster string, obj interface{}) error {
	ns, name, o := m.buildResourceWithIndex(gvr, cluster, obj)
	m.initResourceNamespace(gvr, cluster, ns)
	m.resourceMap.Get(gvr).Get(clusterName(cluster)).Get(namespaceName(ns)).Set(name, o)
	prommonitor.Resources.WithLabelValues(cluster, gvr.Group, gvr.Version, gvr.Resource, ns).
		Set(float64(m.resourceMap.Get(gvr).Get(clusterName(cluster)).Get(namespaceName(ns)).Len()))
	return nil
}

func (m *memoryStore) OnResourceModified(gvr store.GroupVersionResource, cluster string, obj interface{}) error {
	ns, name, o := m.buildResourceWithIndex(gvr, cluster, obj)
	m.initResourceNamespace(gvr, cluster, ns)
	m.resourceMap.Get(gvr).Get(clusterName(cluster)).Get(namespaceName(ns)).Set(name, o)
	prommonitor.Resources.WithLabelValues(cluster, gvr.Group, gvr.Version, gvr.Resource, ns).
		Set(float64(m.resourceMap.Get(gvr).Get(clusterName(cluster)).Get(namespaceName(ns)).Len()))
	return nil
}

func (m *memoryStore) OnResourceDeleted(gvr store.GroupVersionResource, cluster string, obj interface{}) error {
	ns, name, _ := m.buildResourceWithIndex(gvr, cluster, obj)
	m.initResourceNamespace(gvr, cluster, ns)
	m.resourceMap.Get(gvr).Get(clusterName(cluster)).Get(namespaceName(ns)).Delete(name)
	prommonitor.Resources.WithLabelValues(cluster, gvr.Group, gvr.Version, gvr.Resource, ns).
		Set(float64(m.resourceMap.Get(gvr).Get(clusterName(cluster)).Get(namespaceName(ns)).Len()))
	return nil
}

type innerSort struct {
	key     string
	typ     string
	reverse bool
}

func sortObjs(objs []store.Object, s string) ([]store.Object, error) {
	if s == "" {
		s = "cluster, namespace, name"
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
			typ:     constants.KeyTypeStr,
		}
		if strings.Contains(s, " ") {
			parts := strings.Split(s, " ")
			if len(parts) > 2 {
				return objs, nil
			}
			if len(parts) == 2 {
				switch parts[1] {
				case constants.SortDesc:
					st.reverse = true
				case constants.SortASC:
					st.reverse = false
				default:
					return objs, fmt.Errorf("error sort format `%s`", parts[1])
				}
			}
			// override s
			s = parts[0]
		}
		if strings.Contains(s, constants.KeyTypeSep) {
			parts := strings.Split(s, constants.KeyTypeSep)
			if len(parts) != 2 {
				return objs, fmt.Errorf("error type format")
			}
			switch parts[1] {
			case constants.KeyTypeInt:
				st.typ = constants.KeyTypeInt
			case constants.KeyTypeStr:
				st.typ = constants.KeyTypeStr
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
			if s.typ == constants.KeyTypeInt {
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

func (m *memoryStore) Get(gvr store.GroupVersionResource, cluster string, namespace, name string) interface{} {
	if m.resourceMap.Exists(gvr) {
		if !m.resourceMap.Get(gvr).Exists(clusterName(cluster)) {
			return nil
		}
		if !m.resourceMap.Get(gvr).Get(clusterName(cluster)).Exists(namespaceName(namespace)) {
			return nil
		}
		if o := m.resourceMap.Get(gvr).Get(clusterName(cluster)).Get(namespaceName(namespace)).Get(name); o != nil {
			return o.Obj
		}
	}
	return nil
}

func (m *memoryStore) Query(gvr store.GroupVersionResource, query store.Query) store.QueryResult {
	res := store.QueryResult{}
	resources := make([]store.Object, 0)
	m.resourceMap.Get(gvr).ForEach(func(cname clusterName, c *syncResourceStore[
		namespaceName,
		syncResourceStore[string, store.Object],
	]) {
		c.ForEach(func(ns namespaceName, nssObj *syncResourceStore[string, store.Object]) {
			if query.Namespace == "" || query.Namespace == string(ns) {
				nssObj.ForEach(func(name string, obj *store.Object) {
					if ok, err := query.Match(obj.Index); ok {
						resources = append(resources, *obj)
					} else if err != nil {
						res.Error = err
					}
				})
			}
		})
	})
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
	var start, end int64
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

var funMap = map[string]interface{}{
	"default": func(def string, pre interface{}) string {
		if pre == nil {
			return def
		}
		return fmt.Sprintf("%s", pre)
	},
	"quote": func(pre interface{}) string {
		return fmt.Sprintf("%q", pre)
	},
	"join": func(sep string, ins ...string) string {
		return strings.Join(ins, sep)
	},
}

func (m *memoryStore) buildResourceWithIndex(gvr store.GroupVersionResource, cluster string, obj interface{}) (string, string, store.Object) {
	s := store.Object{
		Index: map[string]string{},
		Obj:   obj,
	}
	mobj := utils.Obj2JSONMap(obj)
	jp := jsonpath.New("parser")
	jp.AllowMissingKeys(true)
	gotmpl := template.New("parser").Funcs(funMap)
	for k, v := range m.indexConf[gvr] {
		w := bytes.NewBuffer([]byte{})
		var exec interface {
			Execute(wr io.Writer, data interface{}) error
		}
		var err error
		if strings.Contains(v, "{{") {
			// go template
			exec, err = gotmpl.Parse(v)
		} else if !strings.Contains(v, "{") {
			// raw string
			s.Index[k] = v
			continue
		} else {
			// json path
			_ = jp.Parse(v)
			exec = jp
		}
		if err != nil {
			log.Errorf("parse temp error: %v", err)
			s.Index[k] = w.String()
			continue
		}
		err = exec.Execute(w, mobj)
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
	s.Index["cluster"] = cluster
	if oo, ok := obj.(v1.Object); ok {
		// BUILD-IN Index: deletion
		if oo.GetDeletionTimestamp() != nil {
			s.Index["is_deleted"] = "true"
		} else {
			s.Index["is_deleted"] = "false"
		}
		if len(oo.GetAnnotations()) == 0 {
			oo.SetAnnotations(map[string]string{
				constants.DSMClusterAnno: cluster,
			})
		} else {
			anno := oo.GetAnnotations()
			anno[constants.DSMClusterAnno] = cluster
			oo.SetAnnotations(anno)
		}
		anno := oo.GetAnnotations()
		index, _ := json.Marshal(s.Index)
		anno[constants.IndexAnno] = string(index) // todo constants
		oo.SetAnnotations(anno)
		s.Obj = oo
		namespace = oo.GetNamespace()
		name = oo.GetName()
		s.Index["namespace"] = namespace
		s.Index["name"] = name
	}
	log.Debugf("memory store: gvr: %v, resources %s/%s, index: %v", gvr, namespace, name, s.Index)
	return namespace, name, s
}
