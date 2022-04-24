package main

import (
	"github.com/DaoCloud/ckube/log"
	"github.com/DaoCloud/ckube/page"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"os/exec"
	"strings"
)

const (
	get = iota
	create
	update
	notSupport
)

func main() {
	clusters := ""
	args := []string{}
	typ := notSupport
	selectorPos := 0
	for i := 1; i < len(os.Args); i++ {
		a := os.Args[i]
		switch a {
		case "get":
			typ = get
		case "create":
			typ = create
			//case "delete":
			//	typ = del
		}
		if a == "--clusters" {
			if i+1 < len(os.Args) {
				clusters = os.Args[i+1]
				i++
				continue
			}
		} else {
			args = append(args, a)
		}
		if a == "-l" || a == "--selector" {
			if i+1 < len(os.Args) {
				selectorPos = i + 1
			}
		}
	}
	if typ == notSupport {
		log.Errorf("ckube plugin not support current subcommad")
		os.Exit(1)
	}
	if clusters == "" {
		log.Errorf("you are not specified the --clusters option, we will use the default cluster to process you request")
	} else {
		cs := strings.Split(clusters, ",")
		switch typ {
		case get:
			p := page.Paginate{}
			p.Clusters(cs)
			selector := ""
			if selectorPos != 0 {
				selector = args[selectorPos]
			}
			o, _ := page.QueryListOptions(metav1.ListOptions{
				LabelSelector: selector,
			}, p)
			if selectorPos == 0 {
				args = append(args, "-l", o.LabelSelector)
			} else {
				args[selectorPos] = o.LabelSelector
			}
		case create, update:
			if len(cs) > 1 {
				log.Errorf("create resource can only specified one cluster, error: %v", clusters)
				os.Exit(2)
			}
			p := page.Paginate{}
			p.Clusters(cs)
			o, _ := page.QueryCreateOptions(metav1.CreateOptions{}, cs[0])
			args = append(args, "--field-manager", o.FieldManager)
		}
	}
	c := exec.Command("kubectl", args...)
	//fmt.Printf("args %v\n", args)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Run()
}
