package page

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestQueryListOptions(t *testing.T) {
	cases := []struct {
		name     string
		inOption v1.ListOptions
		inPage   Paginate
		out      v1.ListOptions
	}{
		{
			name: "empty Paginate",
		},
		{
			name: "page",
			inPage: Paginate{
				Page: 1,
			},
			out: v1.ListOptions{
				LabelSelector: "dsm.daocloud.io/query notin (eyJwYWdlIjoxfQ)",
			},
		},
		{
			name: "page & page_size",
			inPage: Paginate{
				Page:     1,
				PageSize: 1,
			},
			out: v1.ListOptions{
				LabelSelector: "dsm.daocloud.io/query notin (eyJwYWdlIjoxLCJwYWdlX3NpemUiOjF9)",
			},
		},
		{
			name: "page & search",
			inPage: Paginate{
				Page:   1,
				Search: "name=ok",
			},
			out: v1.ListOptions{
				LabelSelector: "dsm.daocloud.io/query notin (eyJwYWdlIjoxLCJzZWFyY2giOiJuYW1lPW9rIn0)",
			},
		},
		{
			name: "label",
			inOption: v1.ListOptions{
				LabelSelector: "test=1",
			},
			inPage: Paginate{
				Page:   1,
				Search: "name=ok",
			},
			out: v1.ListOptions{
				LabelSelector: "dsm.daocloud.io/query notin (eyJwYWdlIjoxLCJzZWFyY2giOiJuYW1lPW9rIn0),test=1",
			},
		},
		{
			name: "multi label",
			inOption: v1.ListOptions{
				LabelSelector: "test=1,dsm.daocloud.io/query!=eyJwYWdlIjoxfQ",
			},
			inPage: Paginate{
				Page:   1,
				Search: "name=ok",
			},
			out: v1.ListOptions{
				LabelSelector: "dsm.daocloud.io/query notin (eyJwYWdlIjoxLCJzZWFyY2giOiJuYW1lPW9rIn0),test=1",
			},
		},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("%d---%s", i, c.name), func(t *testing.T) {
			out := QueryListOptions(c.inOption, c.inPage)
			assert.Equal(t, c.out, out)
		})
	}
}
