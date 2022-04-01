package server

//
//import (
//	"github.com/DaoCloud/ckube/store"
//	"github.com/gorilla/mux"
//	"net/http"
//)
//
//type muxServer struct {
//	ListenAddr string
//	router     *mux.Router
//}
//
//func NewMuxServer(listenAddr string) Server {
//	ser := muxServer{
//		ListenAddr: listenAddr,
//		router:     mux.NewRouter(),
//	}
//	ser.router.Path("/apis/{group}/{version}/{resourceType}").HandlerFunc(Proxy)
//	ser.router.Path("/api/{version}/{resourceType}").HandlerFunc(Proxy)
//	ser.router.Path("/apis/{group}/{version}/{resourceType}/{resourceName}").HandlerFunc(Proxy)
//	ser.router.Path("/api/{version}/{resourceType}/{resourceName}").HandlerFunc(Proxy)
//	ser.router.Path("/apis/{group}/{version}/namespaces/{namespace}/{resourceType}").HandlerFunc(Proxy)
//	ser.router.Path("/api/{version}/namespaces/{namespace}/{resourceType}").HandlerFunc(Proxy)
//	ser.router.Path("/apis/{group}/{version}/namespaces/{namespace}/{resourceType}/{resourceName}").HandlerFunc(Proxy)
//	ser.router.Path("/api/{version}/namespaces/{namespace}/{resourceType}/{resourceName}").HandlerFunc(Proxy)
//	return &ser
//}
//
//func Proxy(writer http.ResponseWriter, r *http.Request) {
//	group := mux.Vars(r)["group"]
//	version := mux.Vars(r)["version"]
//	resourceType := mux.Vars(r)["resourceType"]
//	namespace := mux.Vars(r)["namespace"]
//	resourceName := mux.Vars(r)["resourceName"]
//	gvr := store.GroupVersionResource{
//		Group:    group,
//		Version:  version,
//		Resource: resourceType,
//	}
//}
