package fake

import (
	"encoding/json"
	"github.com/DaoCloud/ckube/common"
	"github.com/DaoCloud/ckube/common/constants"
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
)

type fakeCkubeServer struct {
	store      store.Store
	ser        server.Server
	kubeConfig *rest.Config
	eventChan  chan Event
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
	s := fakeCkubeServer{
		store:     m,
		eventChan: make(chan Event),
		kubeConfig: &rest.Config{
			Host: "http://" + func() string {
				parts := strings.Split(listenAddr, ":")
				if parts[0] == "" {
					return listenAddr
				}
				return "127.0.0.1:" + parts[1]
			}(),
		},
	}
	ser := server.NewMuxServer(listenAddr, nil, m, s.registerFakeRoute)
	s.ser = ser
	go ser.Run()
	return &s, nil
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

func (s *fakeCkubeServer) Events() <-chan Event {
	return s.eventChan
}

func (s *fakeCkubeServer) GetKubeConfig() *rest.Config {
	return s.kubeConfig
}

func (s *fakeCkubeServer) registerFakeRoute(r *mux.Router) {
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
		s.store.OnResourceAdded(gvr, cluster, &obj)
	case "PUT":
		action = EventActionUpdate
		s.store.OnResourceModified(gvr, cluster, &obj)
	case "DELETE":
		del := metav1.DeleteOptions{}
		json.Unmarshal(bs, &del)
		if len(del.DryRun) == 1 && strings.HasPrefix(del.DryRun[0], constants.ClusterPrefix) {
			cluster = del.DryRun[0][len(constants.ClusterPrefix):]
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
		Namespace:   namespace,
		Name:        resourceName,
	}
	select {
	case s.eventChan <- e:
	default:
	}
	jsonResp(writer, 200, obj)
}
