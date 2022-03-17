package store

import (
	"github.com/DaoCloud/ckube/page"
)

type Filter func(obj Object) (bool, error)
type Sort func(i, j int) bool

type Query struct {
	Namespace string
	page.Paginate
}

type Store interface {
	IsStoreGVR(gvr GroupVersionResource) bool
	Clean(gvr GroupVersionResource, cluster string) error
	OnResourceAdded(gvr GroupVersionResource, cluster string, obj interface{}) error
	OnResourceModified(gvr GroupVersionResource, cluster string, obj interface{}) error
	OnResourceDeleted(gvr GroupVersionResource, cluster string, obj interface{}) error
	Query(gvr GroupVersionResource, query Query) QueryResult
	Get(gvr GroupVersionResource, cluster string, namespace, name string) interface{}
}
