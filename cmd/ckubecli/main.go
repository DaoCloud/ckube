package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/DaoCloud/ckube/page"
)

func main() {
	var page_ int
	var pageSize int
	var sort string
	var search string
	var clusters string
	flag.IntVar(&page_, "p", 0, "page of result")
	flag.IntVar(&pageSize, "s", 0, "page size of result")
	flag.StringVar(&sort, "sort", "", "sort of result")
	flag.StringVar(&search, "search", "", "search of result")
	flag.StringVar(&clusters, "c", "", "clusters of result, comma splited")
	flag.Parse()
	p := page.Paginate{
		Page:     int64(page_),
		PageSize: int64(pageSize),
		Sort:     sort,
		Search:   search,
	}
	if clusters != "" {
		ccs := strings.Split(clusters, ",")
		_ = p.Clusters(ccs)
	}
	o, err := page.QueryListOptions(v1.ListOptions{}, p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query error: %v", err)
		os.Exit(1)
	}
	fmt.Printf(o.LabelSelector)
}
