package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/DaoCloud/ckube/api"
	"github.com/DaoCloud/ckube/common"
	"github.com/DaoCloud/ckube/log"
	"github.com/DaoCloud/ckube/store"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

type Server interface {
	Run() error
	Stop() error
	ResetStore(store store.Store, clis map[string]kubernetes.Interface)
}

type muxServer struct {
	LogLevel       string
	ListenAddr     string
	router         *mux.Router
	server         *http.Server
	store          store.Store
	clusterClients map[string]kubernetes.Interface
}

type statusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
	w.ResponseWriter.(http.Flusher).Flush()
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	n, err := w.ResponseWriter.Write(b)
	w.ResponseWriter.(http.Flusher).Flush()
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

func NewMuxServer(listenAddr string, clusterClients map[string]kubernetes.Interface, s store.Store, externalRouter ...func(*mux.Router)) Server {
	ser := muxServer{
		clusterClients: clusterClients,
		store:          s,
		ListenAddr:     listenAddr,
		router:         mux.NewRouter(),
	}
	for _, h := range externalRouter {
		h(ser.router)
	}
	ser.registerRoutes(ser.router, routeHandles)
	ser.router.Use(loggingMiddleware)
	ser.router.HandleFunc("/metrics", promhttp.Handler().ServeHTTP).Methods("GET")
	return &ser
}

func (m *muxServer) Run() error {
	m.server = &http.Server{
		Addr:         m.ListenAddr,
		Handler:      m.router,
		ReadTimeout:  30 * time.Minute,
		WriteTimeout: 30 * time.Minute,
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

func (m *muxServer) ResetStore(s store.Store, clis map[string]kubernetes.Interface) {
	m.store = s
	m.clusterClients = clis
}

func parseMethodPath(key string) (method, path string) {
	keys := strings.Split(key, ":")
	if len(keys) > 1 {
		method = keys[0]
		path = strings.Join(keys[1:], ":")
	} else {
		method = "*"
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

func (m *muxServer) registerRoutes(router *mux.Router, handleRoutes []route) {
	for _, r := range handleRoutes {
		func(route route) {
			var rt *mux.Route
			if route.prefix {
				rt = router.PathPrefix(route.path)
			} else {
				rt = router.Path(route.path)
				if r.method != "" {
					rt = rt.Methods(route.method)
				} else {
					rt = rt.Methods("GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD")
				}
			}
			rt.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
				defer func() {
					// deal 500 error
					if err := recover(); err != nil {
						log.Errorf("%s:%s request error: %v", r.Method, route.path, err)
						debug.PrintStack()
						jsonResp(writer, http.StatusInternalServerError, err)
					}
				}()
				if route.authRequired && common.GetConfig().Token != "" {
					if !strings.Contains(r.Header.Get("Authorization"), common.GetConfig().Token) {
						jsonResp(writer, http.StatusUnauthorized, v1.Status{
							Status:  string(v1.StatusReasonUnauthorized),
							Message: "token missing or error",
							Reason:  v1.StatusReason("token missing or error"),
							Code:    401,
						})
						return
					}
				}
				var res interface{}
				res = route.handler(&api.ReqContext{
					ClusterClients: m.clusterClients,
					Store:          m.store,
					Request:        r,
					Writer:         writer,
				})
				if res == nil {
					return
				}
				var status int
				switch res.(type) {
				case error:
					log.Errorf("request return a unexpected error: %v", res)
					panic(res)
				case v1.Status:
					status = int(res.(v1.Status).Code)
				case *v1.Status:
					status = int(res.(*v1.Status).Code)
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
		}(r)
	}
}
