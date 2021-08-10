package api

import (
	"net/http"

	"gitlab.daocloud.cn/mesh/ckube/store"
	"k8s.io/client-go/kubernetes"
)

type ReqContext struct {
	ClusterClients map[string]kubernetes.Interface
	Store          store.Store
	Request        *http.Request
	Writer         http.ResponseWriter
}
