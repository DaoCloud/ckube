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
		Host: "http://127.0.0.1:3033",
	})
	p := page.Paginate{
		// full search
		Search: `name="default"`,
	}
	podList, err := client.CoreV1().Namespaces().List(
		context.Background(),
		page.QueryListOptions(v1.ListOptions{}, p),
	)
	if err != nil {
		panic(err)
	}
	p = page.MakeupResPaginate(podList, p)
	fmt.Printf("total of default namespaces: %d, got %d\n", p.Total, len(podList.Items))
	p = page.Paginate{
		Page:     1,
		PageSize: 5,
		Search:   `name=e`,
	}
	podList, err = client.CoreV1().Namespaces().List(
		context.Background(),
		page.QueryListOptions(v1.ListOptions{}, p),
	)
	if err != nil {
		panic(err)
	}
	p = page.MakeupResPaginate(podList, p)
	fmt.Printf("total of namespaces which name containes e: %d, got %d\n", p.Total, len(podList.Items))
}
