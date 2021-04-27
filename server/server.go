package server

import (
	"context"
	"encoding/json"
	"fmt"
	"gitlab.daocloud.cn/mesh/ckube/api"
	"gitlab.daocloud.cn/dsm-public/common/log"
	"gitlab.daocloud.cn/mesh/ckube/store"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

type Server interface {
	Run() error
	Stop() error
}

type muxServer struct {
	LogLevel   string
	ListenAddr string
	router     *mux.Router
	server     *http.Server
	store      store.Store
	kube       kubernetes.Interface
}

type statusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	n, err := w.ResponseWriter.Write(b)
	w.length += n
	return n, err
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		st := time.Now()
		sw := statusWriter{
			ResponseWriter: w,
			status:         0,
			length:         0,
		}
		next.ServeHTTP(&sw, r)
		log.AccessLog.WithFields(logrus.Fields{
			"type":           "access",
			"path":           r.RequestURI,
			"req_time":       time.Now().Sub(st),
			"status":         sw.status,
			"content_length": sw.length,
		}).Print()
	})
}

func NewMuxServer(listenAddr string, kube kubernetes.Interface, s store.Store) Server {
	ser := muxServer{
		kube:       kube,
		store:      s,
		ListenAddr: listenAddr,
		router:     mux.NewRouter(),
	}
	ser.registerRoutes(ser.router, handleMap)
	ser.router.Use(loggingMiddleware)
	ser.router.HandleFunc("/metrics", promhttp.Handler().ServeHTTP).Methods("GET")
	return &ser
}

func (m *muxServer) Run() error {
	m.server = &http.Server{
		Addr:         m.ListenAddr,
		Handler:      m.router,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}
	log.Infof("starting server at %v", m.ListenAddr)
	return m.server.ListenAndServe()
}

func (m *muxServer) Stop() error {
	if m.server == nil {
		return fmt.Errorf("server not start ever")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	log.Infof("shutting down the server...")
	return m.server.Shutdown(ctx)
}

func parseMethodPath(key string) (method, path string) {
	keys := strings.Split(key, ":")
	if len(keys) > 1 {
		method = keys[0]
		path = strings.Join(keys[1:], ":")
	} else {
		method = "GET"
		path = key
	}
	return
}

func jsonResp(writer http.ResponseWriter, status int, v interface{}) {
	b, _ := json.Marshal(v)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	writer.Write(b)
}

func (m *muxServer) registerRoutes(router *mux.Router, handleMap map[string]route) {
	for k, r := range handleMap {
		func(key string, route route) {
			method, path := parseMethodPath(k)
			var rt *mux.Route
			if route.prefix {
				rt = router.PathPrefix(path)
			} else {
				rt = router.Path(path).Methods(method)
			}
			rt.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
				defer func() {
					// deal 500 error
					if err := recover(); err != nil {
						log.Errorf("%s:%s request error: %v", method, path, err)
						debug.PrintStack()
						jsonResp(writer, http.StatusInternalServerError, nil)
					}
				}()
				var res interface{}
				res = route.handler(&api.ReqContext{
					Kube:    m.kube,
					Store:   m.store,
					Request: r,
					Writer:  writer,
				})
				var status int
				switch res.(type) {
				case error:
					log.Errorf("request return a unexpected error: %v", res)
					panic(res)
				case string:
					writer.Write([]byte(res.(string)))
					return
				case []byte:
					writer.Write(res.([]byte))
					return
				default:
					status = route.successStatus
				}
				jsonResp(writer, status, res)
			})
		}(k, r)
	}
}
