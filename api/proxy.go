package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"gitlab.daocloud.cn/mesh/ckube/common"
	"gitlab.daocloud.cn/mesh/ckube/store"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8labels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
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

func ParseToLabelSelector(selector string) (*v1.LabelSelector, error) {
	reqs, err := k8labels.ParseToRequirements(selector)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse the selector string \"%s\": %v", selector, err)
	}

	labelSelector := &v1.LabelSelector{
		MatchLabels:      map[string]string{},
		MatchExpressions: []v1.LabelSelectorRequirement{},
	}
	for _, req := range reqs {
		var op v1.LabelSelectorOperator
		switch req.Operator() {
		case selection.Equals, selection.DoubleEquals:
			vals := req.Values()
			if vals.Len() != 1 {
				return nil, fmt.Errorf("equals operator must have exactly one value")
			}
			val, ok := vals.PopAny()
			if !ok {
				return nil, fmt.Errorf("equals operator has exactly one value but it cannot be retrieved")
			}
			labelSelector.MatchLabels[req.Key()] = val
			continue
		case selection.In:
			op = v1.LabelSelectorOpIn
		case selection.NotIn, selection.NotEquals:
			op = v1.LabelSelectorOpNotIn
		case selection.Exists:
			op = v1.LabelSelectorOpExists
		case selection.DoesNotExist:
			op = v1.LabelSelectorOpDoesNotExist
		case selection.GreaterThan, selection.LessThan:
			// Adding a separate case for these operators to indicate that this is deliberate
			return nil, fmt.Errorf("%q isn't supported in label selectors", req.Operator())
		default:
			return nil, fmt.Errorf("%q is not a valid label selector operator", req.Operator())
		}
		labelSelector.MatchExpressions = append(labelSelector.MatchExpressions, v1.LabelSelectorRequirement{
			Key:      req.Key(),
			Operator: op,
			Values:   req.Values().List(),
		})
	}
	return labelSelector, nil
}

func Proxy(r *common.ReqContext) interface{} {
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
	var paginate *store.Paginate
	var labels *v1.LabelSelector
	if labelSelectorStr != "" {
		var err error
		labels, err = ParseToLabelSelector(labelSelectorStr)
		if err != nil {
			return err
		}
		if paginateStr, ok := labels.MatchLabels[common.PaginateKey]; ok {
			r, _ := base64.StdEncoding.WithPadding(base64.NoPadding).DecodeString(paginateStr)
			p := store.Paginate{}
			json.Unmarshal(r, &p)
			paginate = &p
			delete(labels.MatchLabels, common.PaginateKey)
		}
	}
	items := make([]interface{}, 0)
	if labels != nil && (len(labels.MatchLabels) != 0 || len(labels.MatchExpressions) != 0) {
		// exists label selector
		res := r.Store.Query(gvr, store.Query{
			Namespace: namespace,
			Paginate:  store.Paginate{}, // get all
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
	} else {
		if paginate == nil {
			paginate = &store.Paginate{}
		}
		res := r.Store.Query(gvr, store.Query{
			Namespace: namespace,
			Paginate:  *paginate,
		})
		items = res.Items
	}
	apiVersion := ""
	if gvr.Group == "" {
		apiVersion = gvr.Version
	} else {
		apiVersion = gvr.Group + "/" + gvr.Version
	}
	return map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       common.GetGVRKind(gvr.Group, gvr.Version, gvr.Resource),
		"metadata": map[string]string{
			"selfLink": r.Request.URL.Path,
			// todo remainingItemCount
			//"remainingItemCount": res.Total -
		},
		"items": items,
	}
}

func proxyPass(r *common.ReqContext) interface{} {

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
