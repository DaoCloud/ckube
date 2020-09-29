package server

import (
	"gitlab.daocloud.cn/mesh/ckube/api/extend"
	"gitlab.daocloud.cn/mesh/ckube/common"
)

type HandleFunc func(r *common.ReqContext) interface{}

type route struct {
	handler       HandleFunc
	authRequired  bool
	adminRequired bool
	successStatus int
	prefix        bool
}

var (
	handleMap = map[string]route{
		"GET:/custom/v1/namespaces/{namespace}/deployments/{deployment}/services": {
			handler:       extend.Deploy2Service,
			authRequired:  true,
			successStatus: 200,
		},
	}
)
