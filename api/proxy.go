package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/gorilla/mux"
	"gitlab.daocloud.cn/mesh/ckube/common"
	"gitlab.daocloud.cn/mesh/ckube/page"
	"gitlab.daocloud.cn/mesh/ckube/store"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8labels "k8s.io/apimachinery/pkg/labels"
	"net/http"
	"reflect"
	"strings"
)

func getGVRFromReq(req *http.Request) store.GroupVersionResource {
	group := mux.Vars(req)["group"]
	version := mux.Vars(req)["version"]
	resourceType := mux.Vars(req)["resourceType"]

	gvr := store.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resourceType,
	}
	return gvr
}

func findLabels(i interface{}) map[string]string {
	meta := reflect.ValueOf(i).Elem().FieldByName("ObjectMeta")
	if !meta.CanInterface() {
		return nil
	}
	metaInterface := meta.Interface()
	labels := reflect.ValueOf(metaInterface).FieldByName("Labels")
	if !labels.CanInterface() {
		return nil
	}
	res := labels.Interface().(map[string]string)
	return res
}

func Proxy(r *ReqContext) interface{} {
	//version := mux.Vars(r.Request)["version"]
	namespace := mux.Vars(r.Request)["namespace"]

	gvr := getGVRFromReq(r.Request)
	if !r.Store.IsStoreGVR(gvr) {
		return proxyPass(r)
	}
	labelSelectorStr := ""
	for k, v := range r.Request.URL.Query() {
		switch k {
		case "labelSelector":
			labelSelectorStr = strings.Join(v, ",")
		case "timeoutSeconds":
		default:
			return proxyPass(r)
		}
	}
	var paginate *page.Paginate
	var labels *v1.LabelSelector
	if labelSelectorStr != "" {
		var err error
		labels, err = common.ParseToLabelSelector(labelSelectorStr)
		if err != nil {
			return err
		}
		if paginateStr, ok := labels.MatchLabels[common.PaginateKey]; ok {
			r, _ := base64.StdEncoding.WithPadding(base64.NoPadding).DecodeString(paginateStr)
			p := page.Paginate{}
			json.Unmarshal(r, &p)
			paginate = &p
			delete(labels.MatchLabels, common.PaginateKey)
		}
	}
	items := make([]interface{}, 0)
	var total int64 = 0
	if labels != nil && (len(labels.MatchLabels) != 0 || len(labels.MatchExpressions) != 0) {
		// exists label selector
		res := r.Store.Query(gvr, store.Query{
			Namespace: namespace,
			Paginate:  page.Paginate{}, // get all
		})
		sel, err := v1.LabelSelectorAsSelector(labels)
		if err != nil {
			return err
		}
		for _, item := range res.Items {
			l := findLabels(item)
			if sel.Matches(k8labels.Set(l)) {
				items = append(items, item)
			}
		}
		total = int64(len(items))
	} else {
		if paginate == nil {
			paginate = &page.Paginate{}
		}
		res := r.Store.Query(gvr, store.Query{
			Namespace: namespace,
			Paginate:  *paginate,
		})
		items = res.Items
		total = res.Total
	}
	apiVersion := ""
	if gvr.Group == "" {
		apiVersion = gvr.Version
	} else {
		apiVersion = gvr.Group + "/" + gvr.Version
	}
	var remainCount int64
	if paginate == nil || (paginate.Page == 0 && paginate.PageSize == 0) {
		// all item returned
		remainCount = 0
	} else {
		// page starts with 1,
		remainCount = total - (paginate.PageSize * paginate.Page)
		if remainCount < 0 {
			remainCount = 0
		}
	}
	return map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       common.GetGVRKind(gvr.Group, gvr.Version, gvr.Resource),
		"metadata": map[string]interface{}{
			"selfLink":           r.Request.URL.Path,
			"remainingItemCount": remainCount,
		},
		"items": items,
	}
}

func proxyPass(r *ReqContext) interface{} {

	ls := r.Request.URL.Query().Get("labelSelector")
	if ls != "" {
		parts := strings.Split(ls, ",")
		pp := []string{}
		for _, part := range parts {
			if strings.HasPrefix(part, common.PaginateKey) {
				continue
			}
			pp = append(pp, part)
		}
		r.Request.URL.Query().Set("labelSelector", strings.Join(pp, ","))
	}

	res, err := r.Kube.Discovery().RESTClient().Get().RequestURI(r.Request.URL.String()).DoRaw(context.Background())
	r.Writer.Header().Set("Content-Type", "application/json")
	if err != nil {
		if es, ok := err.(*errors.StatusError); ok {
			r.Writer.WriteHeader(int(es.ErrStatus.Code))
			return res
		}
		return err
	}
	return res
}
