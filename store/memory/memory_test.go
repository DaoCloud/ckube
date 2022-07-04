package memory

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/DaoCloud/ckube/common/constants"
	"github.com/DaoCloud/ckube/page"
	"github.com/DaoCloud/ckube/store"
)

var podsGVR = store.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "pods",
}

var depsGVR = store.GroupVersionResource{
	Group:    "apps",
	Version:  "v1",
	Resource: "deployments",
}

var testIndexConf = map[store.GroupVersionResource]map[string]string{
	podsGVR: {
		"namespace": "{.metadata.namespace}",
		"name":      "{.metadata.name}",
		"uid":       "{.metadata.uid}",
	},
	depsGVR: {
		"namespace": "{.metadata.namespace}",
		"name":      "{.metadata.name}",
		"uid":       "{.metadata.uid}",
		"replicas":  "{.spec.replicas}",
	},
}

func TestMemoryStore_Query(t *testing.T) {
	cases := []struct {
		name      string
		gvr       store.GroupVersionResource
		resources []runtime.Object
		query     store.Query
		res       store.QueryResult
	}{
		{
			name: "page & pagesize",
			gvr:  podsGVR,
			resources: append([]runtime.Object{}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
					UID:       "test",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test2",
					Namespace: "test",
					UID:       "test",
				},
			}),
			query: store.Query{
				Namespace: "test",
				Paginate: page.Paginate{
					Page:     1,
					PageSize: 1,
				},
			},
			res: store.QueryResult{
				Error: nil,
				Items: append([]interface{}{}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test1",
						Namespace: "test",
						UID:       "test",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test1\",\"namespace\":\"test\",\"uid\":\"test\"}",
						},
					},
				}),
				Total: 2,
			},
		},
		{
			name: "page & pagesize 2",
			gvr:  podsGVR,
			resources: append([]runtime.Object{}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
					UID:       "test",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test2",
					Namespace: "test",
					UID:       "test",
				},
			}),
			query: store.Query{
				Namespace: "test",
				Paginate: page.Paginate{
					Page:     2,
					PageSize: 1,
				},
			},
			res: store.QueryResult{
				Error: nil,
				Items: append([]interface{}{}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test2",
						Namespace: "test",
						UID:       "test",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test2\",\"namespace\":\"test\",\"uid\":\"test\"}",
						},
					},
				}),
				Total: 2,
			},
		},
		{
			name: "fuzzy search",
			gvr:  podsGVR,
			resources: append([]runtime.Object{}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "test",
					UID:       "test",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llo",
					Namespace: "test",
					UID:       "test",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "l1lo",
					Namespace: "test",
					UID:       "test",
				},
			}),
			query: store.Query{
				Namespace: "test",
				Paginate: page.Paginate{
					Search: "llo",
				},
			},
			res: store.QueryResult{
				Error: nil,
				Items: append([]interface{}{}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hello",
						Namespace: "test",
						UID:       "test",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"hello\",\"namespace\":\"test\",\"uid\":\"test\"}",
						},
					},
				}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "llo",
						Namespace: "test",
						UID:       "test",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"llo\",\"namespace\":\"test\",\"uid\":\"test\"}",
						},
					},
				}),
				Total: 2,
			},
		},
		{
			name: "full search",
			gvr:  podsGVR,
			resources: append([]runtime.Object{}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "test",
					UID:       "test",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llo",
					Namespace: "test",
					UID:       "test",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "l1lo",
					Namespace: "test",
					UID:       "test",
				},
			}),
			query: store.Query{
				Namespace: "test",
				Paginate: page.Paginate{
					Search: "name=\"llo\"",
				},
			},
			res: store.QueryResult{
				Error: nil,
				Items: append([]interface{}{}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "llo",
						Namespace: "test",
						UID:       "test",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"llo\",\"namespace\":\"test\",\"uid\":\"test\"}",
						},
					},
				}),
				Total: 1,
			},
		},
		{
			name: "search key missing",
			gvr:  podsGVR,
			resources: append([]runtime.Object{}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "test",
					UID:       "test",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llo",
					Namespace: "test",
					UID:       "test",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "l1lo",
					Namespace: "test",
					UID:       "test",
				},
			}),
			query: store.Query{
				Namespace: "test",
				Paginate: page.Paginate{
					Search: "name1=llo",
				},
			},
			res: store.QueryResult{
				Error: fmt.Errorf("unexpected search key: name1"),
				Total: 0,
			},
		},
		{
			name: "advance search",
			gvr:  podsGVR,
			resources: append([]runtime.Object{}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "test",
					UID:       "test",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llo",
					Namespace: "test",
					UID:       "test",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "l1lo",
					Namespace: "test",
					UID:       "test",
				},
			}),
			query: store.Query{
				Namespace: "test",
				Paginate: page.Paginate{
					Search: "__ckube_as__:name in (hello, l1lo)",
				},
			},
			res: store.QueryResult{
				Error: nil,
				Items: append([]interface{}{}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hello",
						Namespace: "test",
						UID:       "test",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"hello\",\"namespace\":\"test\",\"uid\":\"test\"}",
						},
					},
				}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "l1lo",
						Namespace: "test",
						UID:       "test",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"l1lo\",\"namespace\":\"test\",\"uid\":\"test\"}",
						},
					},
				}),
				Total: 2,
			},
		},
		{
			name: "advance search key missing",
			gvr:  podsGVR,
			resources: append([]runtime.Object{}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "test",
					UID:       "test",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llo",
					Namespace: "test",
					UID:       "test",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "l1lo",
					Namespace: "test",
					UID:       "test",
				},
			}),
			query: store.Query{
				Namespace: "test",
				Paginate: page.Paginate{
					Search: "__ckube_as__:test in (hello, l1lo)",
				},
			},
			res: store.QueryResult{
				Error: nil,
				Total: 0,
			},
		},
		{
			name: "advance search format error",
			gvr:  podsGVR,
			resources: append([]runtime.Object{}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "test",
					UID:       "test",
				},
			}),
			query: store.Query{
				Namespace: "test",
				Paginate: page.Paginate{
					Search: "__ckube_as__:test xxx",
				},
			},
			res: store.QueryResult{
				Error: fmt.Errorf("couldn't parse the selector string \"test xxx\": unable to parse requirement: found 'xxx', expected: '=', '!=', '==', 'in', notin'"),
				Total: 0,
			},
		},
		{
			name: "sort single key",
			gvr:  podsGVR,
			resources: append([]runtime.Object{}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
					UID:       "2",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test5",
					Namespace: "test",
					UID:       "1",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test3",
					Namespace: "test",
					UID:       "3",
				},
			}),
			query: store.Query{
				Namespace: "test",
				Paginate: page.Paginate{
					Sort: "uid!str",
				},
			},
			res: store.QueryResult{
				Error: nil,
				Items: append([]interface{}{}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test5",
						Namespace: "test",
						UID:       "1",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test5\",\"namespace\":\"test\",\"uid\":\"1\"}",
						},
					},
				}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test1",
						Namespace: "test",
						UID:       "2",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test1\",\"namespace\":\"test\",\"uid\":\"2\"}",
						},
					},
				}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test3",
						Namespace: "test",
						UID:       "3",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test3\",\"namespace\":\"test\",\"uid\":\"3\"}",
						},
					},
				}),
				Total: 3,
			},
		},
		{
			name: "sort single key desc",
			gvr:  podsGVR,
			resources: append([]runtime.Object{}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
					UID:       "2",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test5",
					Namespace: "test",
					UID:       "1",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test3",
					Namespace: "test",
					UID:       "3",
				},
			}),
			query: store.Query{
				Namespace: "test",
				Paginate: page.Paginate{
					Sort: "uid desc",
				},
			},
			res: store.QueryResult{
				Error: nil,
				Items: append([]interface{}{}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test3",
						Namespace: "test",
						UID:       "3",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test3\",\"namespace\":\"test\",\"uid\":\"3\"}",
						},
					},
				}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test1",
						Namespace: "test",
						UID:       "2",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test1\",\"namespace\":\"test\",\"uid\":\"2\"}",
						},
					},
				}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test5",
						Namespace: "test",
						UID:       "1",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test5\",\"namespace\":\"test\",\"uid\":\"1\"}",
						},
					},
				}),
				Total: 3,
			},
		},
		{
			name: "sort invalid key convert",
			gvr:  podsGVR,
			resources: append([]runtime.Object{}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
					UID:       "2",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test5",
					Namespace: "test",
					UID:       "1",
				},
			}),
			query: store.Query{
				Namespace: "test",
				Paginate: page.Paginate{
					Sort: "name!int",
				},
			},
			res: store.QueryResult{
				Error: fmt.Errorf("value of `name` can not convert to number"),
				Total: 0,
			},
		},
		{
			name: "multiple keys desc",
			gvr:  podsGVR,
			resources: append([]runtime.Object{}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
					UID:       "2",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test5",
					Namespace: "test1",
					UID:       "1",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test3",
					Namespace: "test",
					UID:       "3",
				},
			}),
			query: store.Query{
				Namespace: "",
				Paginate: page.Paginate{
					Sort: "namespace,uid desc",
				},
			},
			res: store.QueryResult{
				Error: nil,
				Items: append([]interface{}{}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test3",
						Namespace: "test",
						UID:       "3",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test3\",\"namespace\":\"test\",\"uid\":\"3\"}",
						},
					},
				}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test1",
						Namespace: "test",
						UID:       "2",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test1\",\"namespace\":\"test\",\"uid\":\"2\"}",
						},
					},
				}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test5",
						Namespace: "test1",
						UID:       "1",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test5\",\"namespace\":\"test1\",\"uid\":\"1\"}",
						},
					},
				}),
				Total: 3,
			},
		},
		{
			name: "sort type",
			gvr:  podsGVR,
			resources: append([]runtime.Object{}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
					UID:       "11",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test5",
					Namespace: "test",
					UID:       "2",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test3",
					Namespace: "test",
					UID:       "3",
				},
			}),
			query: store.Query{
				Namespace: "",
				Paginate: page.Paginate{
					Sort: "uid!int",
				},
			},
			res: store.QueryResult{
				Error: nil,
				Items: append([]interface{}{}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test5",
						Namespace: "test",
						UID:       "2",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test5\",\"namespace\":\"test\",\"uid\":\"2\"}",
						},
					},
				}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test3",
						Namespace: "test",
						UID:       "3",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test3\",\"namespace\":\"test\",\"uid\":\"3\"}",
						},
					},
				}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test1",
						Namespace: "test",
						UID:       "11",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test1\",\"namespace\":\"test\",\"uid\":\"11\"}",
						},
					},
				}),
				Total: 3,
			},
		},
		{
			name: "union search",
			gvr:  podsGVR,
			resources: append([]runtime.Object{}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
					UID:       "11",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ok",
					Namespace: "test",
					UID:       "2",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "0tes",
					Namespace: "test",
					UID:       "3",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test3",
					Namespace: "test",
					UID:       "3",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test3",
					Namespace: "test1",
					UID:       "30",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test13",
					Namespace: "test1",
					UID:       "20",
				},
			}, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ok",
					Namespace: "test1",
					UID:       "2",
				},
			}),
			query: store.Query{
				Namespace: "",
				Paginate: page.Paginate{
					Page:     1,
					PageSize: 3,
					Search:   "name=test; __ckube_as__:name notin (ok)",
					Sort:     "namespace,uid!int",
				},
			},
			res: store.QueryResult{
				Error: nil,
				Items: append([]interface{}{}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test3",
						Namespace: "test",
						UID:       "3",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test3\",\"namespace\":\"test\",\"uid\":\"3\"}",
						},
					},
				}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test1",
						Namespace: "test",
						UID:       "11",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test1\",\"namespace\":\"test\",\"uid\":\"11\"}",
						},
					},
				}, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test13",
						Namespace: "test1",
						UID:       "20",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test13\",\"namespace\":\"test1\",\"uid\":\"20\"}",
						},
					},
				}),
				Total: 4,
			},
		},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("%d-%s", i, c.name), func(t *testing.T) {
			s := NewMemoryStore(testIndexConf)
			for _, r := range c.resources {
				_ = s.OnResourceAdded(c.gvr, "", r)
			}
			res := s.Query(c.gvr, c.query)
			assert.Equal(t, c.res, res)
		})
	}
}
