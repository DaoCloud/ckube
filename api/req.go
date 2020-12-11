package api

import (
	"gitlab.daocloud.cn/mesh/ckube/store"
	"k8s.io/client-go/kubernetes"
	"net/http"
)

type ReqContext struct {
	Kube    kubernetes.Interface
	Store   store.Store
	Request *http.Request
	Writer  http.ResponseWriter
}
