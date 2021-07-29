package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.daocloud.cn/mesh/ckube/common"
	"gitlab.daocloud.cn/mesh/ckube/store"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

type fakeWriter struct {
	bs   []byte
	code int
}

func (f *fakeWriter) Header() http.Header {
	return http.Header{}
}

func (f *fakeWriter) Write(bytes []byte) (int, error) {
	f.bs = bytes
	return len(bytes), nil
}

func (f *fakeWriter) WriteHeader(statusCode int) {
	f.code = statusCode
}

type fakeStore struct {
	store.Store
	storeResources store.QueryResult
}

func (f fakeStore) Query(gvr store.GroupVersionResource, query store.Query) store.QueryResult {
	return f.storeResources
}

func (f fakeStore) IsStoreGVR(gvr store.GroupVersionResource) bool {
	return gvr.Group == "" && gvr.Version == "v1" && gvr.Resource == "pods"
}

type fakeValueContext struct {
	context.Context
	resultMap map[string]string
}

func (c fakeValueContext) Value(key interface{}) interface{} {
	return c.resultMap
}

var podsMap = map[string]string{
	"namespace":    "default",
	"group":        "",
	"version":      "v1",
	"resourceType": "pods",
}

var nsMap = map[string]string{
	"namespace":    "default",
	"group":        "",
	"version":      "v1",
	"resourceType": "pods",
}

func podsInterfaces(pods []v1.Pod) []interface{} {
	a := []interface{}{}
	for _, p := range pods {
		a = append(a, p.DeepCopy())
	}
	return a
}

var testPods = podsInterfaces([]v1.Pod{
	{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	},
})

func TestProxy(t *testing.T) {
	common.InitConfig(&common.Config{Proxies: []common.Proxy{
		{
			Group:    "",
			Version:  "v1",
			Resource: "pods",
			ListKind: "PodList",
		},
	}})
	cases := []struct {
		name           string
		path           string
		storeResources store.QueryResult
		contextMap     map[string]string
		kubeResources  []runtime.Object
		expectCode     int
		expectRes      interface{}
	}{
		{
			name:       "query pods",
			path:       "/api/v1/pods",
			contextMap: podsMap,
			storeResources: store.QueryResult{
				Items: testPods,
				Total: 1,
			},
			expectCode: 0,
			expectRes: map[string]interface{}(
				map[string]interface{}{
					"apiVersion": "v1",
					"items":      testPods,
					"kind":       "PodList",
					"metadata":   map[string]interface{}{"remainingItemCount": int64(0), "selfLink": "/api/v1/pods"}}),
		},
		{
			name:       "query pods with label selector",
			path:       "/api/v1/pods?labelSelector=test=1",
			contextMap: podsMap,
			storeResources: store.QueryResult{
				Items: podsInterfaces([]v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Labels: map[string]string{
								"test": "1",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test1",
							Labels: map[string]string{
								"test": "2",
							},
						},
					},
				}),
				Total: 2,
			},
			expectCode: 0,
			expectRes: map[string]interface{}(
				map[string]interface{}{
					"apiVersion": "v1",
					"items": podsInterfaces([]v1.Pod{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test",
								Labels: map[string]string{
									"test": "1",
								},
							},
						},
					}),
					"kind":     "PodList",
					"metadata": map[string]interface{}{"remainingItemCount": int64(0), "selfLink": "/api/v1/pods"}}),
		},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("%d---%s", i, c.name), func(t *testing.T) {
			req, _ := http.NewRequestWithContext(
				fakeValueContext{resultMap: c.contextMap},
				"GET",
				c.path,
				nil,
			)
			s := fakeStore{
				storeResources: c.storeResources,
			}
			client := fake.NewSimpleClientset(c.kubeResources...)
			writer := fakeWriter{}
			res := Proxy(&ReqContext{
				ClusterClients: map[string]kubernetes.Interface{"": client},
				Store:          s,
				Request:        req,
				Writer:         &writer,
			})
			assert.Equal(t, c.expectCode, writer.code)
			assert.Equal(t, c.expectRes, res)
		})
	}
}
