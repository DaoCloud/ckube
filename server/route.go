package server

import (
	"gitlab.daocloud.cn/mesh/ckube/api"
	"gitlab.daocloud.cn/mesh/ckube/api/extend"
	"gitlab.daocloud.cn/mesh/ckube/utils/prommonitor"
)

type HandleFunc func(r *api.ReqContext) interface{}

type route struct {
	path          string
	method        string
	handler       HandleFunc
	authRequired  bool
	adminRequired bool
	successStatus int
	prefix        bool
}

var (
	routeHandles = []route{
		// healthy
		{
			path:   "/healthy",
			method: "GET",
			handler: func(r *api.ReqContext) interface{} {
				r.Writer.Write([]byte("1"))
				r.Writer.WriteHeader(200)
				return nil
			},
		},
		// metrics url
		{
			path:    "/metrics",
			method:  "GET",
			handler: prommonitor.PromHandler,
		},
		{
			path:          "/custom/v1/namespaces/{namespace}/deployments/{deployment}/services",
			method:        "GET",
			handler:       extend.Deploy2Service,
			authRequired:  true,
			successStatus: 200,
		},
		{
			path:          "/apis/{group}/{version}/namespaces/{namespace}/{resourceType}",
			method:        "GET",
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		{
			path:          "/apis/{group}/{version}/{resourceType}",
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		{
			path:          "/api/{version}/{resourceType}",
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		{
			path:          "/api/{version}/namespaces/{namespace}/{resourceType}",
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},

		// single resources
		{
			path:          "/apis/{group}/{version}/namespaces/{namespace}/{resourceType}/{resource}",
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		{
			path:          "/apis/{group}/{version}/{resourceType}/{resource}",
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		{
			path:          "/api/{version}/{resourceType}/{resource}",
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		{
			path:          "/api/{version}/namespaces/{namespace}/{resourceType}/{resource}",
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
		{
			path:          "/",
			prefix:        true,
			handler:       api.Proxy,
			authRequired:  true,
			successStatus: 200,
		},
	}
)
