package server

import (
	"gitlab.daocloud.cn/mesh/ckube/api"
	"gitlab.daocloud.cn/mesh/ckube/api/extend"
	"gitlab.daocloud.cn/mesh/ckube/utils/prommonitor"
)

type HandleFunc func(r *api.ReqContext) interface{}

type route struct {
	handler       HandleFunc
	authRequired  bool
	adminRequired bool
	successStatus int
	prefix        bool
}

var (
	handleMap = map[string]route{
		// healthy
		"GET:/healthy": {
			handler: func(r *api.ReqContext) interface{} {
				r.Writer.Write([]byte("1"))
				r.Writer.WriteHeader(200)
				return nil
			},
		},
		// metrics url
		"GET:/metrics": {
			handler: prommonitor.PromHandler,
		},
		"GET:/custom/v1/namespaces/{namespace}/deployments/{deployment}/services": {
			handler:       extend.Deploy2Service,
			authRequired:  true,
			successStatus: 200,
		},
		"/apis/{group}/{version}/namespaces/{namespace}/{resourceType}": {
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		"/apis/{group}/{version}/{resourceType}": {
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		"/api/{version}/{resourceType}": {
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		"/api/{version}/namespaces/{namespace}/{resourceType}": {
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},

		// single resources
		"/apis/{group}/{version}/namespaces/{namespace}/{resourceType}/{resource}": {
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		"/apis/{group}/{version}/{resourceType}/{resource}": {
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		"/api/{version}/{resourceType}/{resource}": {
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		"/api/{version}/namespaces/{namespace}/{resourceType}/{resource}": {
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		"/version": {
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
	}
)
