package server

import (
	"gitlab.daocloud.cn/mesh/ckube/api"
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
		"GET:/apis/{group}/{version}/namespaces/{namespace}/{resourceType}": {
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		"GET:/apis/{group}/{version}/{resourceType}": {
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		"GET:/api/{version}/{resourceType}": {
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		"GET:/api/{version}/namespaces/{namespace}/{resourceType}": {
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
	}
)
