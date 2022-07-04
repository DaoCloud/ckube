package extend

import (
	"strings"

	"github.com/gorilla/mux"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/DaoCloud/ckube/api"
	"github.com/DaoCloud/ckube/common"
	"github.com/DaoCloud/ckube/page"
	"github.com/DaoCloud/ckube/store"
	"github.com/DaoCloud/ckube/utils"
	"github.com/DaoCloud/ckube/watcher"
)

func Deploy2Service(r *api.ReqContext) interface{} {
	cluster := mux.Vars(r.Request)["cluster"]
	ns := mux.Vars(r.Request)["namespace"]
	dep := mux.Vars(r.Request)["deployment"]
	services := []*v1.Service{}
	podGvr := store.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}
	svcGvr := store.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}
	if cluster == "" {
		cluster = common.GetConfig().DefaultCluster
	}
	p := page.Paginate{Search: "name=" + dep}
	_ = p.Clusters([]string{cluster})
	res := r.Store.Query(podGvr, store.Query{
		Namespace: ns,
		Paginate:  p,
	})
	if res.Error != nil {
		return res.Error
	}
	var labels map[string]string
	for _, podIf := range res.Items {
		if pod, ok := podIf.(v12.Object); ok {
			depName := getDeploymentName(pod)
			if depName != "" {
				labels = pod.GetLabels()
				break
			}
		}
	}
	p = page.Paginate{}
	_ = p.Clusters([]string{cluster})
	res = r.Store.Query(svcGvr, store.Query{
		Namespace: ns,
		Paginate:  p,
	})
	if res.Error != nil {
		return res.Error
	}
	for _, svcIf := range res.Items {
		svc := &v1.Service{}
		if s, ok := svcIf.(*watcher.ObjType); ok {
			bs, _ := json.Marshal(s)
			_ = json.Unmarshal(bs, svc)
		} else {
			svc = svcIf.(*v1.Service)
		}
		if svc.Spec.Selector != nil && utils.IsSubsetOf(svc.Spec.Selector, labels) {
			services = append(services, svc)
		}
	}
	return services
}

func getDeploymentName(pod v12.Object) string {
	if len(pod.GetOwnerReferences()) == 0 || pod.GetOwnerReferences()[0].Kind != "ReplicaSet" {
		return ""
	} else {
		parts := strings.Split(pod.GetOwnerReferences()[0].Name, "-")
		return strings.Join(parts[:len(parts)-1], "-")
	}
}
