package extend

import (
	"github.com/gorilla/mux"
	"gitlab.daocloud.cn/mesh/ckube/common"
	"gitlab.daocloud.cn/mesh/ckube/store"
	"gitlab.daocloud.cn/mesh/ckube/utils"
	v1 "k8s.io/api/core/v1"
	"strings"
)

func Deploy2Service(r *common.ReqContext) interface{} {
	ns := mux.Vars(r.Request)["namespace"]
	dep := mux.Vars(r.Request)["deployment"]
	services := []*v1.Service{}
	podGvr := store.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
		ListKind: "PodList",
	}
	svcGvr := store.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
		ListKind: "ServiceList",
	}
	res := r.Store.Query(podGvr, store.Query{
		Namespace: ns,
		Paginate: store.Paginate{
			Search: "name=" + dep,
		},
	})
	if res.Error != nil {
		return res.Error
	}
	var labels map[string]string
	for _, podIf := range res.Items {
		if pod, ok := podIf.(*v1.Pod); ok {
			depName := getDeploymentName(pod)
			if depName != "" {
				labels = pod.Labels
				break
			}
		}
	}
	res = r.Store.Query(svcGvr, store.Query{
		Namespace: ns,
		Paginate:  store.Paginate{},
	})
	if res.Error != nil {
		return res.Error
	}
	for _, svcIf := range res.Items {
		if svc, ok := svcIf.(*v1.Service); ok {
			if svc.Spec.Selector != nil && utils.IsSubsetOf(svc.Spec.Selector, labels) {
				services = append(services, svc)
			}
		}
	}
	return services
}

func getDeploymentName(pod *v1.Pod) string {
	if len(pod.OwnerReferences) == 0 || pod.OwnerReferences[0].Kind != "ReplicaSet" {
		return ""
	} else {
		parts := strings.Split(pod.OwnerReferences[0].Name, "-")
		return strings.Join(parts[:len(parts)-1], "-")
	}
}
