package main

import (
	"context"
	"fmt"
	"gitlab.daocloud.cn/mesh/ckube/page"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	client := kubernetes.NewForConfigOrDie(&rest.Config{
		Host:    "http://127.0.0.1:3033",
		APIPath: "",
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	})
	p := page.Paginate{
		Page:     2,
		PageSize: 50,
	}
	podList, err := client.CoreV1().Pods("").List(
		context.Background(),
		page.QueryListOptions(v1.ListOptions{}, p),
	)
	if err != nil {
		panic(err)
	}
	p = page.MakeupResPaginate(podList, p)
	fmt.Printf("total of pods: %d, got %d pods", p.Total, len(podList.Items))
}
