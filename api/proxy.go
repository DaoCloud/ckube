package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/gorilla/mux"
	"gitlab.daocloud.cn/dsm-public/common/constants"
	"gitlab.daocloud.cn/dsm-public/common/kube"
	"gitlab.daocloud.cn/dsm-public/common/log"
	"gitlab.daocloud.cn/dsm-public/common/page"
	"gitlab.daocloud.cn/mesh/ckube/common"
	"gitlab.daocloud.cn/mesh/ckube/store"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8labels "k8s.io/apimachinery/pkg/labels"
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
		meta = reflect.ValueOf(i).Elem().FieldByName("metadata")
		if !meta.CanInterface() {
			return nil
		}
	}
	metaInterface := meta.Interface()
	labels := reflect.ValueOf(metaInterface).FieldByName("Labels")
	if !labels.CanInterface() {
		labels = reflect.ValueOf(metaInterface).FieldByName("labels")
		if !labels.CanInterface() {
			return nil
		}
	}
	res := labels.Interface().(map[string]string)
	return res
}

func errorProxy(w http.ResponseWriter, err v1.Status) interface{} {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(int(err.Code))
	err.Kind = "Status"
	err.APIVersion = "v1"
	return err
}

func ProxySingleResources(r *ReqContext, gvr store.GroupVersionResource, namespace, resource string) interface{} {
	cluster := ""
	clusterPrefix := "dsm-cluster-"
	if rv, ok := r.Request.URL.Query()["resourceVersion"]; ok && len(rv) == 1 && strings.HasPrefix(rv[0], clusterPrefix) {
		cluster = rv[0][len(clusterPrefix):]
	} else if ok {
		return proxyPass(r, cluster)
	}
	res := r.Store.Get(gvr, cluster, namespace, resource) // todo
	if res == nil {
		return errorProxy(r.Writer, v1.Status{
			Status:  v1.StatusFailure,
			Message: fmt.Sprintf("resource %v: %s/%s/%s not found", gvr, cluster, namespace, resource),
			Reason:  v1.StatusReasonNotFound,
			Details: nil,
			Code:    404,
		})
	}
	return res
}

func parsePaginateAndLabels(labelSelectorStr string) (*page.Paginate, *v1.LabelSelector, error) {
	var labels *v1.LabelSelector
	var paginate page.Paginate
	if labelSelectorStr != "" {
		var err error
		labels, err = kube.ParseToLabelSelector(labelSelectorStr)
		if err != nil {
			return nil, nil, err
		}
		paginateStr := ""
		if ps, ok := labels.MatchLabels[constants.PaginateKey]; ok {
			paginateStr = ps
			delete(labels.MatchLabels, constants.PaginateKey)
		} else {
			mes := []v1.LabelSelectorRequirement{}
			// Why we use MatchExpressions?
			// to adapt dsm.daocloud.io/query=xxxx send to apiserver, which makes no results.
			// if dsm.daocloud.io/query != xxx or dsm.daocloud.io/query not in (xxx), results exist even if it was sent to apiserver.
			for _, m := range labels.MatchExpressions {
				if m.Key == constants.PaginateKey {
					if len(m.Values) > 0 {
						paginateStr, err = kube.MergeValues(m.Values)
						if err != nil {
							return nil, labels, err
						}
					}
				} else {
					mes = append(mes, m)
				}
			}
			labels.MatchExpressions = mes
		}
		if paginateStr != "" {
			rr, err := base64.StdEncoding.WithPadding(base64.NoPadding).DecodeString(paginateStr)
			if err != nil {
				return nil, labels, err
			}
			json.Unmarshal(rr, &paginate)
			delete(labels.MatchLabels, constants.PaginateKey)
		}
	}
	return &paginate, labels, nil
}

func Proxy(r *ReqContext) interface{} {
	//version := mux.Vars(r.Request)["version"]
	namespace := mux.Vars(r.Request)["namespace"]
	resourceName := mux.Vars(r.Request)["resource"]

	gvr := getGVRFromReq(r.Request)
	if resourceName != "" {
		return ProxySingleResources(r, gvr, namespace, resourceName)
	}
	labelSelectorStr := ""
	var paginate *page.Paginate
	var labels *v1.LabelSelector
	var err error
	for k, v := range r.Request.URL.Query() {
		switch k {
		case "labelSelector":
			labelSelectorStr = strings.Join(v, ",")
			paginate, labels, err = parsePaginateAndLabels(labelSelectorStr)
			if err != nil {
				return proxyPass(r, common.GetConfig().DefaultCluster)
			}
		default:
			log.Warnf("got unexpected query key: %s, value: %v, proxyPass to api server", k, v)
			return proxyPass(r, common.GetConfig().DefaultCluster)
		case "timeoutSeconds":
		case "timeout":
		}
	}
	if paginate == nil {
		paginate = &page.Paginate{}
	}
	if !r.Store.IsStoreGVR(gvr) {
		log.Debugf("gvr %v no cached", gvr)
		if cs := paginate.GetClusters(); len(cs) == 1 {
			return proxyPass(r, cs[0])
		}
		return proxyPass(r, common.GetConfig().DefaultCluster)
	}
	// default only get default cluster's resources,
	// If you want to get all clusters' resources,
	// please call paginate.Clusters() before fetch resources
	if cs := paginate.GetClusters(); len(cs) == 0 {
		err = paginate.Clusters([]string{common.GetConfig().DefaultCluster})
		if err != nil {
			log.Errorf("set cluster error: %v", err)
		}
	}

	items := make([]interface{}, 0)
	var total int64 = 0
	if labels != nil && (len(labels.MatchLabels) != 0 || len(labels.MatchExpressions) != 0) {
		// exists label selector
		res := r.Store.Query(gvr, store.Query{
			Namespace: namespace,
			Paginate: page.Paginate{
				Sort:   paginate.Sort,
				Search: paginate.Search,
			}, // get all
		})
		if res.Error != nil {
			return errorProxy(r.Writer, v1.Status{
				Status:  v1.StatusFailure,
				Message: "query error",
				Reason:  v1.StatusReason(res.Error.Error()),
				Code:    400,
			})
		}
		sel, err := v1.LabelSelectorAsSelector(labels)
		if err != nil {
			return errorProxy(r.Writer, v1.Status{
				Status:  v1.StatusFailure,
				Message: "label selector parse error",
				Reason:  v1.StatusReason(err.Error()),
				Details: nil,
				Code:    400,
			})
		}
		for _, item := range res.Items {
			l := findLabels(item)
			if sel.Matches(k8labels.Set(l)) {
				items = append(items, item)
			}
		}

		// manually slice items
		var l = int64(len(items))
		var start, end int64 = 0, 0
		if paginate.PageSize == 0 || paginate.Page == 0 {
			// all resources
			start = 0
			end = l
		} else {
			start = (paginate.Page - 1) * paginate.PageSize
			end = start + paginate.PageSize
			if start >= l {
				start = l
			}
			if end >= l {
				end = l
			}
		}
		items = items[start:end]
		total = l
	} else {
		res := r.Store.Query(gvr, store.Query{
			Namespace: namespace,
			Paginate:  *paginate,
		})
		if res.Error != nil {
			return errorProxy(r.Writer, v1.Status{
				Status:  v1.StatusFailure,
				Message: "query error",
				Reason:  v1.StatusReason(res.Error.Error()),
				Code:    400,
			})
		}
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
	if paginate.Page == 0 && paginate.PageSize == 0 {
		// all item returned
		remainCount = 0
	} else {
		// page starts with 1,
		remainCount = total - (paginate.PageSize * paginate.Page)
		if remainCount < 0 && len(items) == 0 && paginate.Page != 1 {
			return errorProxy(r.Writer, v1.Status{
				Status:  v1.StatusFailure,
				Message: "out of page",
				Reason:  v1.StatusReason(fmt.Sprintf("request resources out of page: %d", remainCount)),
				Code:    400,
			})
		} else if remainCount < 0 {
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

func proxyPass(r *ReqContext, cluster string) interface{} {

	ls := r.Request.URL.Query().Get("labelSelector")
	if ls != "" {
		parts := strings.Split(ls, ",")
		pp := []string{}
		for _, part := range parts {
			if strings.HasPrefix(part, constants.PaginateKey) {
				continue
			}
			pp = append(pp, part)
		}
		r.Request.URL.Query().Set("labelSelector", strings.Join(pp, ","))
	}
	u := r.Request.URL.String()
	log.Debugf("proxyPass url: %s", u)
	res, err := r.ClusterClients[cluster].Discovery().RESTClient().Get().RequestURI(u).DoRaw(context.Background())
	if err != nil {
		if es, ok := err.(*errors.StatusError); ok {
			return errorProxy(r.Writer, es.ErrStatus)
		}
		return err
	}
	r.Writer.Header().Set("Content-Type", "application/json")
	return res
}
