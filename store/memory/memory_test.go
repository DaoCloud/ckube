package memory

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/DaoCloud/ckube/common/constants"
	"github.com/DaoCloud/ckube/page"
	"github.com/DaoCloud/ckube/store"
)

var podsGVR = store.GroupVersionResource{
	Group:    "",
	Version:  "corev1",
	Resource: "pods",
}

var depsGVR = store.GroupVersionResource{
	Group:    "apps",
	Version:  "corev1",
	Resource: "deployments",
}

func TestConcurrentReadWrite(t *testing.T) {
	m := NewMemoryStore(map[store.GroupVersionResource]map[string]string{
		podsGVR: {
			"name": "{.metadata.name}",
		},
	})
	wg := sync.WaitGroup{}
	round := 1000
	wg.Add(round)
	go func() {
		for i := 0; i < round; i++ {
			_ = m.OnResourceAdded(podsGVR, "test", &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("test-%d", i),
					Namespace: "test",
				},
			})
			wg.Done()
		}
	}()
	wg.Add(round)
	go func() {
		for i := 0; i < round; i++ {
			_ = m.OnResourceDeleted(podsGVR, "test", &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("test-%d", i),
					Namespace: "test",
				},
			})
			wg.Done()
		}
	}()
	wg.Add(round)
	go func() {
		for i := 0; i < round; i++ {
			m.Get(podsGVR, "test", "test", fmt.Sprintf("test-%d", i))
			wg.Done()
		}
	}()
	wg.Wait()
}

func BenchmarkWrite(b *testing.B) {
	m := NewMemoryStore(map[store.GroupVersionResource]map[string]string{
		podsGVR: {
			"name": "{.metadata.name}",
		},
	})
	for i := 0; i < b.N; i++ {
		_ = m.OnResourceAdded(podsGVR, "test", &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-%d", i),
				Namespace: "test",
			},
		})
	}
}

func BenchmarkRead(b *testing.B) {
	m := NewMemoryStore(map[store.GroupVersionResource]map[string]string{
		podsGVR: {
			"name": "{.metadata.name}",
		},
	})
	go func() {
		for i := 0; i < b.N; i++ {
			_ = m.OnResourceAdded(podsGVR, "test", &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("test-%d", i),
					Namespace: "test",
				},
			})
		}
	}()
	for i := 0; i < b.N; i++ {
		m.Get(podsGVR, "test", "test", fmt.Sprintf("test-%d", i))
	}
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
			resources: append([]runtime.Object{}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
					UID:       "test",
				},
			}, &corev1.Pod{
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
				Items: append([]interface{}{}, &corev1.Pod{
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
			resources: append([]runtime.Object{}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
					UID:       "test",
				},
			}, &corev1.Pod{
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
				Items: append([]interface{}{}, &corev1.Pod{
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
			resources: append([]runtime.Object{}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "test",
					UID:       "test",
				},
			}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llo",
					Namespace: "test",
					UID:       "test",
				},
			}, &corev1.Pod{
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
				Items: append([]interface{}{}, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hello",
						Namespace: "test",
						UID:       "test",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"hello\",\"namespace\":\"test\",\"uid\":\"test\"}",
						},
					},
				}, &corev1.Pod{
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
			resources: append([]runtime.Object{}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "test",
					UID:       "test",
				},
			}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llo",
					Namespace: "test",
					UID:       "test",
				},
			}, &corev1.Pod{
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
				Items: append([]interface{}{}, &corev1.Pod{
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
			resources: append([]runtime.Object{}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "test",
					UID:       "test",
				},
			}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llo",
					Namespace: "test",
					UID:       "test",
				},
			}, &corev1.Pod{
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
			resources: append([]runtime.Object{}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "test",
					UID:       "test",
				},
			}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llo",
					Namespace: "test",
					UID:       "test",
				},
			}, &corev1.Pod{
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
				Items: append([]interface{}{}, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hello",
						Namespace: "test",
						UID:       "test",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"hello\",\"namespace\":\"test\",\"uid\":\"test\"}",
						},
					},
				}, &corev1.Pod{
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
			resources: append([]runtime.Object{}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "test",
					UID:       "test",
				},
			}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llo",
					Namespace: "test",
					UID:       "test",
				},
			}, &corev1.Pod{
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
			resources: append([]runtime.Object{}, &corev1.Pod{
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
			resources: append([]runtime.Object{}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
					UID:       "2",
				},
			}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test5",
					Namespace: "test",
					UID:       "1",
				},
			}, &corev1.Pod{
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
				Items: append([]interface{}{}, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test5",
						Namespace: "test",
						UID:       "1",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test5\",\"namespace\":\"test\",\"uid\":\"1\"}",
						},
					},
				}, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test1",
						Namespace: "test",
						UID:       "2",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test1\",\"namespace\":\"test\",\"uid\":\"2\"}",
						},
					},
				}, &corev1.Pod{
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
			resources: append([]runtime.Object{}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
					UID:       "2",
				},
			}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test5",
					Namespace: "test",
					UID:       "1",
				},
			}, &corev1.Pod{
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
				Items: append([]interface{}{}, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test3",
						Namespace: "test",
						UID:       "3",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test3\",\"namespace\":\"test\",\"uid\":\"3\"}",
						},
					},
				}, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test1",
						Namespace: "test",
						UID:       "2",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test1\",\"namespace\":\"test\",\"uid\":\"2\"}",
						},
					},
				}, &corev1.Pod{
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
			resources: append([]runtime.Object{}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
					UID:       "2",
				},
			}, &corev1.Pod{
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
			resources: append([]runtime.Object{}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
					UID:       "2",
				},
			}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test5",
					Namespace: "test1",
					UID:       "1",
				},
			}, &corev1.Pod{
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
				Items: append([]interface{}{}, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test3",
						Namespace: "test",
						UID:       "3",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test3\",\"namespace\":\"test\",\"uid\":\"3\"}",
						},
					},
				}, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test1",
						Namespace: "test",
						UID:       "2",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test1\",\"namespace\":\"test\",\"uid\":\"2\"}",
						},
					},
				}, &corev1.Pod{
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
			resources: append([]runtime.Object{}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
					UID:       "11",
				},
			}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test5",
					Namespace: "test",
					UID:       "2",
				},
			}, &corev1.Pod{
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
				Items: append([]interface{}{}, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test5",
						Namespace: "test",
						UID:       "2",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test5\",\"namespace\":\"test\",\"uid\":\"2\"}",
						},
					},
				}, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test3",
						Namespace: "test",
						UID:       "3",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test3\",\"namespace\":\"test\",\"uid\":\"3\"}",
						},
					},
				}, &corev1.Pod{
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
			resources: append([]runtime.Object{}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "test",
					UID:       "11",
				},
			}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ok",
					Namespace: "test",
					UID:       "2",
				},
			}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "0tes",
					Namespace: "test",
					UID:       "3",
				},
			}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test3",
					Namespace: "test",
					UID:       "3",
				},
			}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test3",
					Namespace: "test1",
					UID:       "30",
				},
			}, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test13",
					Namespace: "test1",
					UID:       "20",
				},
			}, &corev1.Pod{
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
				Items: append([]interface{}{}, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test3",
						Namespace: "test",
						UID:       "3",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test3\",\"namespace\":\"test\",\"uid\":\"3\"}",
						},
					},
				}, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test1",
						Namespace: "test",
						UID:       "11",
						Annotations: map[string]string{
							constants.DSMClusterAnno: "",
							constants.IndexAnno:      "{\"cluster\":\"\",\"is_deleted\":\"false\",\"name\":\"test1\",\"namespace\":\"test\",\"uid\":\"11\"}",
						},
					},
				}, &corev1.Pod{
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

func TestMemoryStore_buildResourceWithIndex(t *testing.T) {
	cases := []struct {
		name          string
		index         map[string]string
		obj           interface{}
		expectedIndex map[string]string
	}{
		{
			name: "jsonpath",
			index: map[string]string{
				"namespace":  "{.metadata.namespace}",
				"name":       "{.metadata.name}",
				"containers": "{.spec.containers[*].name}",
				"status":     "{.status.phase}",
			},
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-1",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "c1"},
						{Name: "c2"},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			expectedIndex: map[string]string{
				"namespace":  "default",
				"name":       "test-1",
				"containers": "c1 c2",
				"status":     "Running",
			},
		},
		{
			name: "go tmpl",
			index: map[string]string{
				"status":         `{{if .metadata.deletionTimestamp }}Deleting{{else}}{{.status.phase}}{{end}}`,
				"default status": `{{ .x | default "no spec"}}`,
				"quote":          `{{ .status.phase | quote }}`,
				"join":           `{{ join "/" .metadata.namespace .metadata.name }}`,
				"raw":            "test raw",
			},
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-1",
					Namespace: "default",
					DeletionTimestamp: &metav1.Time{
						Time: time.Now(),
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			expectedIndex: map[string]string{
				"join":           "default/test-1",
				"status":         "Deleting",
				"default status": "no spec",
				"quote":          "\"Running\"",
				"raw":            "test raw",
				"name":           "test-1",
				"namespace":      "default",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gvr := store.GroupVersionResource{}
			m := memoryStore{
				indexConf: map[store.GroupVersionResource]map[string]string{
					gvr: c.index,
				},
			}
			_, _, o := m.buildResourceWithIndex(gvr, "test", c.obj)
			delete(o.Index, "is_deleted")
			delete(o.Index, "cluster")
			assert.Equal(t, c.expectedIndex, o.Index)
		})
	}
}
