package fake

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/DaoCloud/ckube/common"
	"github.com/DaoCloud/ckube/common/constants"
	"github.com/DaoCloud/ckube/kube"
	"github.com/DaoCloud/ckube/log"
	"github.com/DaoCloud/ckube/page"
	"github.com/DaoCloud/ckube/server"
	"github.com/DaoCloud/ckube/store"
	"github.com/DaoCloud/ckube/store/memory"
	"github.com/DaoCloud/ckube/watcher"
	"github.com/gorilla/mux"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type fakeCkubeServer struct {
	store         store.Store
	ser           server.Server
	kubeConfig    *rest.Config
	eventChan     chan Event
	watchChanMap  map[string]chan Event
	watchChanLock sync.RWMutex
}

func NewFakeCKubeServer(listenAddr string, config string) (CkubeServer, error) {
	cfg := common.Config{}
	err := json.Unmarshal([]byte(config), &cfg)
	if err != nil {
		return nil, err
	}
	cfg.Token = ""
	common.InitConfig(&cfg)
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
	addr := "http://" + func() string {
		parts := strings.Split(listenAddr, ":")
		if parts[0] == "" {
			return listenAddr
		}
		return "127.0.0.1:" + parts[1]
	}()
	s := fakeCkubeServer{
		store:     m,
		eventChan: make(chan Event),
		kubeConfig: &rest.Config{
			Host: addr,
		},
		watchChanMap: make(map[string]chan Event),
	}
	ser := server.NewMuxServer(listenAddr, nil, m, s.registerFakeRoute)
	s.ser = ser
	go ser.Run()
	for i := 0; i < 5; i++ {
		time.Sleep(time.Millisecond * 100 * time.Duration(1<<i))
		_, err := http.Get(addr + "/metrics")
		if err == nil {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("wait for server start timeout")
}

func NewFakeCKubeServerWithConfigPath(listenAddr string, cfgPath string) (CkubeServer, error) {
	bs, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}
	return NewFakeCKubeServer(listenAddr, string(bs))
}

func (s *fakeCkubeServer) Stop() {
	s.ser.Stop()
}

func (s *fakeCkubeServer) Clean() {
	indexConf := map[store.GroupVersionResource]map[string]string{}
	storeGVRConfig := []store.GroupVersionResource{}
	for _, proxy := range common.GetConfig().Proxies {
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
	s.ser.ResetStore(m, nil)
	s.store = m
}

func (s *fakeCkubeServer) Events() <-chan Event {
	return s.eventChan
}

func (s *fakeCkubeServer) GetKubeConfig() *rest.Config {
	return s.kubeConfig
}

func (s *fakeCkubeServer) registerFakeRoute(r *mux.Router) {
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wq := r.URL.Query()["watch"]
			if len(wq) == 1 && (wq[0] == "true" || wq[0] == "1") {
				h := wh{s: s}
				h.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	for _, p := range []string{
		"/apis/{group}/{version}/{resourceType}",
		"/api/{version}/{resourceType}",
		"/apis/{group}/{version}/{resourceType}/{resourceName}",
		"/api/{version}/{resourceType}/{resourceName}",
		"/apis/{group}/{version}/namespaces/{namespace}/{resourceType}",
		"/api/{version}/namespaces/{namespace}/{resourceType}",
		"/apis/{group}/{version}/namespaces/{namespace}/{resourceType}/{resourceName}",
		"/api/{version}/namespaces/{namespace}/{resourceType}/{resourceName}",
	} {
		r.Path(p).Methods("POST", "PUT", "DELETE").HandlerFunc(s.proxy)
	}
	for _, p := range []string{
		// watch
		"/apis/{group}/{version}/watch/{resourceType}",
		"/api/{version}/watch/{resourceType}",
		"/apis/{group}/{version}/watch/namespaces/{namespace}/{resourceType}",
		"/api/{version}/watch/namespaces/{namespace}/{resourceType}",
	} {
		r.Path(p).Methods("GET").HandlerFunc(s.watch)
	}
	r.Path("/version").Methods("GET").HandlerFunc(version)
	r.Path("/api").Methods("GET").HandlerFunc(api)
	r.Path("/apis").Methods("GET").HandlerFunc(apis)
	r.Path("/apis/{group}/{version}").Methods("GET").HandlerFunc(resources)
	r.Path("/api/{version}").Methods("GET").HandlerFunc(resources)
}

func jsonResp(writer http.ResponseWriter, status int, v interface{}) {
	b, _ := json.Marshal(v)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	writer.Write(b)
}

func errorProxy(w http.ResponseWriter, err metav1.Status) {
	jsonResp(w, int(err.Code), err)
}

type wh struct {
	s *fakeCkubeServer
}

func (w *wh) ServeHTTP(writer http.ResponseWriter, r *http.Request) {
	w.s.watch(writer, r)
}

func resources(writer http.ResponseWriter, r *http.Request) {
	group := mux.Vars(r)["group"]
	version := mux.Vars(r)["version"]
	res := metav1.APIResourceList{}
	if group == "" {
		res.GroupVersion = version
	} else {
		res.GroupVersion = fmt.Sprintf("%s/%s", group, version)
	}
	for _, p := range common.GetConfig().Proxies {
		res.APIResources = append(res.APIResources, metav1.APIResource{
			Name:         p.Resource,
			SingularName: "",
			Namespaced:   true,
			Group:        group,
			Version:      version,
			Kind:         strings.TrimSuffix(p.ListKind, "List"),
			Verbs: metav1.Verbs{
				"create",
				"delete",
				"deletecollection",
				"get",
				"list",
				"patch",
				"update",
				"watch",
			},
		})
	}
	jsonResp(writer, 200, res)
}

func api(writer http.ResponseWriter, r *http.Request) {
	a := metav1.APIVersions{
		Versions: []string{"v1"},
	}
	jsonResp(writer, 200, a)
}

func apis(writer http.ResponseWriter, r *http.Request) {
	a := metav1.APIGroupList{}

	for _, p := range common.GetConfig().Proxies {
		find := false
		for _, g := range a.Groups {
			if g.Name == p.Group {
				g.Versions = append(g.Versions, metav1.GroupVersionForDiscovery{
					GroupVersion: fmt.Sprintf("%s/%s", p.Group, p.Version),
					Version:      p.Version,
				})
				find = true
				break
			}
		}
		if !find {
			a.Groups = append(a.Groups, metav1.APIGroup{
				Name:                       p.Group,
				Versions:                   nil,
				PreferredVersion:           metav1.GroupVersionForDiscovery{},
				ServerAddressByClientCIDRs: nil,
			})
		}
	}
	jsonResp(writer, 200, a)
}

func version(writer http.ResponseWriter, r *http.Request) {
	// todo fixme
	v := map[string]string{
		"major":        "1",
		"minor":        "23",
		"gitVersion":   "v1.23.5",
		"gitCommit":    "c285e781331a3785a7f436042c65c5641ce8a9e9",
		"gitTreeState": "clean",
		"buildDate":    "2022-03-16T15:52:18Z",
		"goVersion":    "go1.17.8",
		"compiler":     "gc",
		"platform":     "linux/amd64",
	}
	jsonResp(writer, 200, v)
}

func (s *fakeCkubeServer) watch(writer http.ResponseWriter, r *http.Request) {
	group := mux.Vars(r)["group"]
	version := mux.Vars(r)["version"]
	resourceType := mux.Vars(r)["resourceType"]
	namespace := mux.Vars(r)["namespace"]

	query := r.URL.Query()
	labelSelectorStr := ""
	for k, v := range query {
		switch k {
		case "labelSelector": // For List options
			labelSelectorStr = v[0]
		}
	}
	paginate := page.Paginate{}
	if labelSelectorStr != "" {
		var err error
		labels, _ := kube.ParseToLabelSelector(labelSelectorStr)
		paginateStr := ""
		if ps, ok := labels.MatchLabels[constants.PaginateKey]; ok {
			paginateStr = ps
			delete(labels.MatchLabels, constants.PaginateKey)
		} else {
			mes := []metav1.LabelSelectorRequirement{}
			// Why we use MatchExpressions?
			// to adapt dsm.daocloud.io/query=xxxx send to apiserver, which makes no results.
			// if dsm.daocloud.io/query != xxx or dsm.daocloud.io/query not in (xxx), results exist even if it was sent to apiserver.
			for _, m := range labels.MatchExpressions {
				if m.Key == constants.PaginateKey {
					if len(m.Values) > 0 {
						paginateStr, err = kube.MergeValues(m.Values)
						if err != nil {
							errorProxy(writer, metav1.Status{
								Message: err.Error(),
								Code:    400,
							})
							return
						}
					}
				} else {
					mes = append(mes, m)
				}
			}
			labels.MatchExpressions = mes
		}
		if paginateStr != "" {
			rr, err := base64.StdEncoding.WithPadding(base64.NoPadding).DecodeString(paginateStr)
			if err != nil {
				errorProxy(writer, metav1.Status{
					Message: err.Error(),
					Code:    400,
				})
				return
			}
			json.Unmarshal(rr, &paginate)
			delete(labels.MatchLabels, constants.PaginateKey)
		}
	}

	s.watchChanLock.Lock()
	s.watchChanMap[r.RemoteAddr] = make(chan Event)
	s.watchChanLock.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Transfer-Encoding", "chunked")
	writer.Header().Set("Connection", "keep-alive")
	writer.(http.Flusher).Flush()
	for {
		select {
		case <-ctx.Done():
			s.watchChanLock.Lock()
			close(s.watchChanMap[r.RemoteAddr])
			delete(s.watchChanMap, r.RemoteAddr)
			s.watchChanLock.Unlock()
			return
		case e := <-s.watchChanMap[r.RemoteAddr]:
			if e.Group == group &&
				e.Version == version &&
				e.Resource == resourceType &&
				(e.Namespace == namespace || namespace == "") {

				if len(paginate.GetClusters()) > 0 && !func() bool {
					for _, c := range paginate.GetClusters() {
						if e.Cluster == c {
							return true
						}
					}
					return false
				}() {
					continue
				}
				typ := "ERROR"
				switch e.EventAction {
				case EventActionAdd:
					typ = "ADDED"
				case EventActionDelete:
					typ = "DELETED"
				case EventActionUpdate:
					typ = "MODIFIED"
				}
				res := fmt.Sprintf(`{"type": %q, "object": %s}`, typ, e.Raw)
				writer.Write([]byte(res + "\n"))
				writer.(http.Flusher).Flush()
			}
		}
	}
}

func (s *fakeCkubeServer) proxy(writer http.ResponseWriter, r *http.Request) {
	group := mux.Vars(r)["group"]
	version := mux.Vars(r)["version"]
	resourceType := mux.Vars(r)["resourceType"]
	namespace := mux.Vars(r)["namespace"]
	resourceName := mux.Vars(r)["resourceName"]
	gvr := store.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resourceType,
	}

	query := r.URL.Query()
	cluster := common.GetConfig().DefaultCluster
	for k, v := range query {
		switch k {
		case "fieldManager", "resourceVersion": // For Get Create Patch Update actions.
			if strings.HasPrefix(v[0], constants.ClusterPrefix) {
				cluster = v[0][len(constants.ClusterPrefix):]
			}
		case "watch":
			if strings.ToLower(v[0]) == "true" || strings.ToLower(v[0]) == "1" {
				s.watch(writer, r)
				return
			}
		}
	}
	obj := watcher.ObjType{}
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		errorProxy(writer, metav1.Status{
			Message: err.Error(),
			Code:    500,
		})
		return
	}
	json.Unmarshal(bs, &obj)
	obj.Namespace = namespace
	if resourceName == "" {
		resourceName = obj.Name
	}
	action := EventActionError
	switch r.Method {
	case "POST":
		action = EventActionAdd
		if o := s.store.Get(gvr, cluster, namespace, resourceName); o != nil {
			errorProxy(writer, metav1.Status{
				Message: fmt.Sprintf("resource %v %s %s/%s already exists", gvr, cluster, namespace, resourceName),
				Code:    400,
			})
			return
		}
		s.store.OnResourceAdded(gvr, cluster, &obj)
	case "PUT":
		action = EventActionUpdate
		if o := s.store.Get(gvr, cluster, namespace, resourceName); o == nil {
			errorProxy(writer, metav1.Status{
				Message: fmt.Sprintf("resource %v %s %s/%s not found", gvr, cluster, namespace, resourceName),
				Code:    404,
			})
			return
		}
		s.store.OnResourceModified(gvr, cluster, &obj)
	case "DELETE":
		action = EventActionDelete
		del := metav1.DeleteOptions{}
		json.Unmarshal(bs, &del)
		if len(del.DryRun) == 1 && strings.HasPrefix(del.DryRun[0], constants.ClusterPrefix) {
			cluster = del.DryRun[0][len(constants.ClusterPrefix):]
		}
		if o := s.store.Get(gvr, cluster, namespace, resourceName); o == nil {
			errorProxy(writer, metav1.Status{
				Message: fmt.Sprintf("resource %v %s %s/%s not found", gvr, cluster, namespace, resourceName),
				Code:    404,
			})
			return
		} else {
			bs, _ = json.Marshal(o)
		}
		obj.Name = resourceName
		obj.Namespace = namespace
		s.store.OnResourceDeleted(gvr, cluster, &obj)
	}
	e := Event{
		EventAction: action,
		Group:       group,
		Version:     version,
		Resource:    resourceType,
		Cluster:     cluster,
		Namespace:   namespace,
		Name:        resourceName,
		Raw:         string(bs),
	}
	select {
	case s.eventChan <- e:
	default:
	}
	s.watchChanLock.RLock()
	for remote, c := range s.watchChanMap {
		select {
		case c <- e:
			log.Debugf("succeed send stream to %s", remote)
		default:
			log.Infof("remote watcher %s no active stream", remote)
		}
	}
	s.watchChanLock.RUnlock()
	jsonResp(writer, 200, obj)
}
