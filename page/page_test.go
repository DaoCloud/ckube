package page

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestQueryListOptions(t *testing.T) {
	cases := []struct {
		name       string
		inOption   v1.ListOptions
		inPage     Paginate
		out        v1.ListOptions
		wannaError bool
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
			out, err := QueryListOptions(c.inOption, c.inPage)
			assert.Equal(t, c.out, out)
			if c.wannaError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPaginate_Match(t *testing.T) {
	cases := []struct {
		name  string
		index map[string]string
		p     Paginate
		match bool
		err   error
	}{
		{
			name: "match fuzzy",
			index: map[string]string{
				"name": "test",
				"ok":   "qq",
			},
			p: Paginate{
				Search: "qq",
			},
			match: true,
			err:   nil,
		},
		{
			name: "match fuzzy 2",
			index: map[string]string{
				"name": "test",
				"ok":   "qq",
			},
			p: Paginate{
				Search: "test",
			},
			match: true,
			err:   nil,
		},
		{
			name: "contains",
			index: map[string]string{
				"name": "test",
				"ok":   "qq",
			},
			p: Paginate{
				Search: "name=est",
			},
			match: true,
			err:   nil,
		},
		{
			name: "full match",
			index: map[string]string{
				"name": "test",
				"ok":   "qq",
			},
			p: Paginate{
				Search: "name=\"test\"",
			},
			match: true,
			err:   nil,
		},
		{
			name: "full match 2",
			index: map[string]string{
				"name": "test",
				"ok":   "qq",
			},
			p: Paginate{
				Search: "name=\"est\"",
			},
			match: false,
			err:   nil,
		},
		{
			name: "advance match",
			index: map[string]string{
				"name": "test",
				"ok":   "qq",
			},
			p: Paginate{
				Search: "__ckube_as__: name in (test)",
			},
			match: true,
			err:   nil,
		},
		{
			name: "advance match not equal",
			index: map[string]string{
				"name": "test",
				"ok":   "qq",
			},
			p: Paginate{
				Search: "__ckube_as__: name != test",
			},
			match: false,
			err:   nil,
		},
		{
			name: "multiple",
			index: map[string]string{
				"name": "test",
				"ok":   "qq",
			},
			p: Paginate{
				Search: "ok=qq; __ckube_as__: name=test",
			},
			match: true,
			err:   nil,
		},
		{
			name: "key error",
			index: map[string]string{
				"name": "test",
				"ok":   "qq",
			},
			p: Paginate{
				Search: "xx=test",
			},
			match: false,
			err:   fmt.Errorf("unexpected search key: xx"),
		},
		{
			name: "not key contains",
			index: map[string]string{
				"name": "test",
				"ok":   "qq",
			},
			p: Paginate{
				Search: "name=!test",
			},
			match: false,
		},
		{
			name: "not contains",
			index: map[string]string{
				"name": "test",
				"ok":   "qq",
			},
			p: Paginate{
				Search: "!test",
			},
			match: false,
		},
		{
			name: "not contains 2",
			index: map[string]string{
				"name": "test",
				"ok":   "qq",
			},
			p: Paginate{
				Search: "!xxx",
			},
			match: true,
		},
		{
			name: "simbol",
			index: map[string]string{
				"name": "a;b",
				"ok":   "qq",
			},
			p: Paginate{
				Search: "a;;",
			},
			match: true,
		},
		{
			name: "simbol and not contains",
			index: map[string]string{
				"name": "a;b",
				"ok":   "qq",
			},
			p: Paginate{
				Search: "a;;;!qq",
			},
			match: false,
		},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("%d-%s", i, c.name), func(t *testing.T) {
			match, err := c.p.Match(c.index)
			assert.Equal(t, c.match, match)
			assert.Equal(t, c.err, err)
		})
	}
}

func TestPaginate_Namespaces(t *testing.T) {
	p := Paginate{}
	err := p.Namespaces([]string{"test", "test1"})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "__ckube_as__:namespace in (test,test1)", p.Search)
	p = Paginate{
		Search: "test=ok",
	}
	err = p.Namespaces([]string{"test", "test1"})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "__ckube_as__:namespace in (test,test1);test=ok", p.Search)
	err = p.Namespaces([]string{})
	if err == nil {
		t.Fatal("must be error")
	}
}
