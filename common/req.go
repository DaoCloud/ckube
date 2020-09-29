package common

import (
	"gitlab.daocloud.cn/mesh/ckube/store"
	"net/http"
)

type ReqContext struct {
	Store   store.Store
	Request *http.Request
	Writer  http.ResponseWriter
}
