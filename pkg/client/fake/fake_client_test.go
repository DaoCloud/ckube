package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/DaoCloud/ckube/page"
)

func TestNewFakeCKubeServer(t *testing.T) {
	s, err := NewFakeCKubeServer(":65521", `
{
  "proxies": [
    {
      "group": "",
      "version": "v1",
      "resource": "pods",
      "list_kind": "PodList",
      "index": {
        "namespace": "{.metadata.namespace}",
        "name": "{.metadata.name}",
        "labels": "{.metadata.labels}",
        "created_at": "{.metadata.creationTimestamp}"
      }
    },
    {
      "group": "",
      "version": "v1",
      "resource": "services",
      "list_kind": "ServiceList",
      "index": {
        "namespace": "{.metadata.namespace}",
        "name": "{.metadata.name}",
        "labels": "{.metadata.labels}",
        "created_at": "{.metadata.creationTimestamp}"
      }
    }
  ]
}
`)
	assert.NoError(t, err)
	defer s.Stop()
	cfb := s.GetKubeConfig()
	cli, err := kubernetes.NewForConfig(cfb)
	assert.NoError(t, err)
	t.Run("create pods", func(t *testing.T) {
		coptc1, _ := page.QueryCreateOptions(metav1.CreateOptions{}, "c1")
		coptc2, _ := page.QueryCreateOptions(metav1.CreateOptions{}, "c2")
		_, err = cli.CoreV1().Pods("test").Create(context.Background(), &v1.Pod{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod1",
				Namespace: "test",
			},
			Spec: v1.PodSpec{
				DNSPolicy: "ClusterFirst",
			},
		}, coptc1)
		assert.NoError(t, err)
		_, err = cli.CoreV1().Pods("test").Create(context.Background(), &v1.Pod{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod2",
				Namespace: "test",
			},
			Spec: v1.PodSpec{
				DNSPolicy: "ClusterFirst",
			},
		}, coptc2)
		assert.NoError(t, err)
	})
	t.Run("get pod each cluster", func(t *testing.T) {
		goptc1, _ := page.QueryGetOptions(metav1.GetOptions{}, "c1")
		goptc2, _ := page.QueryGetOptions(metav1.GetOptions{}, "c2")
		p1, err := cli.CoreV1().Pods("test").Get(context.Background(), "pod1", goptc1)
		assert.NoError(t, err)
		assert.Equal(t, v1.DNSPolicy("ClusterFirst"), p1.Spec.DNSPolicy)
		_, err = cli.CoreV1().Pods("test").Get(context.Background(), "pod1", goptc2)
		assert.Error(t, err)
	})
	t.Run("list pods", func(t *testing.T) {
		p := page.Paginate{}
		_ = p.Clusters([]string{"c1", "c2"})
		lopts, _ := page.QueryListOptions(metav1.ListOptions{}, p)
		pods, err := cli.CoreV1().Pods("test").List(context.Background(), lopts)
		assert.NoError(t, err)
		assert.Len(t, pods.Items, 2)
	})
	t.Run("update pods", func(t *testing.T) {
		uoptc1, _ := page.QueryUpdateOptions(metav1.UpdateOptions{}, "c1")
		_, err := cli.CoreV1().Pods("test").Update(context.Background(), &v1.Pod{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod1",
				Namespace: "test",
			},
			Spec: v1.PodSpec{
				DNSPolicy: "Default",
			},
		}, uoptc1)
		assert.NoError(t, err)
		goptc1, _ := page.QueryGetOptions(metav1.GetOptions{}, "c1")
		p1, err := cli.CoreV1().Pods("test").Get(context.Background(), "pod1", goptc1)
		assert.NoError(t, err)
		assert.Equal(t, v1.DNSPolicy("Default"), p1.Spec.DNSPolicy)
	})
	t.Run("delete pods", func(t *testing.T) {
		doptc1, _ := page.QueryDeleteOptions(metav1.DeleteOptions{}, "c1")
		err := cli.CoreV1().Pods("test").Delete(context.Background(), "pod1", doptc1)
		assert.NoError(t, err)
		goptc1, _ := page.QueryGetOptions(metav1.GetOptions{}, "c1")
		_, err = cli.CoreV1().Pods("test").Get(context.Background(), "pod1", goptc1)
		assert.Error(t, err)
	})

	t.Run("events", func(t *testing.T) {
		events := []Event{}
		lock := sync.Mutex{}
		go func() {
			for e := range s.Events() {
				lock.Lock()
				events = append(events, e)
				lock.Unlock()
			}
		}()
		coptc1, _ := page.QueryCreateOptions(metav1.CreateOptions{}, "c1")
		_, err = cli.CoreV1().Pods("test").Create(context.Background(), &v1.Pod{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod1",
				Namespace: "test",
			},
			Spec: v1.PodSpec{
				DNSPolicy: "ClusterFirst",
			},
		}, coptc1)
		assert.NoError(t, err)
		lock.Lock()
		defer lock.Unlock()
		assert.Equal(t, []Event{
			{
				EventAction: EventActionAdd,
				Group:       "",
				Version:     "v1",
				Resource:    "pods",
				Cluster:     "c1",
				Namespace:   "test",
				Name:        "pod1",
				Raw:         "{\"kind\":\"Pod\",\"apiVersion\":\"v1\",\"metadata\":{\"name\":\"pod1\",\"namespace\":\"test\",\"creationTimestamp\":null},\"spec\":{\"containers\":null,\"dnsPolicy\":\"ClusterFirst\"},\"status\":{}}\n",
			},
		}, events)
	})
	t.Run("watch", func(t *testing.T) {
		events := []Event{}
		lock := sync.Mutex{}
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			p := page.Paginate{}
			_ = p.Clusters([]string{"c1"})
			lopts, _ := page.QueryListOptions(metav1.ListOptions{}, p)
			w, err := cli.CoreV1().Pods("test").Watch(context.Background(), lopts)
			assert.NoError(t, err)
			wg.Done()
			for e := range w.ResultChan() {
				pod := e.Object.(*v1.Pod)
				lock.Lock()
				events = append(events, Event{
					Cluster:   page.GetObjectCluster(pod),
					Namespace: pod.Namespace,
					Name:      pod.Name,
				})
				lock.Unlock()
			}
		}()
		wg.Wait()
		coptc1, _ := page.QueryCreateOptions(metav1.CreateOptions{}, "c1")
		_, err = cli.CoreV1().Pods("test").Create(context.Background(), &v1.Pod{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod3",
				Namespace: "test",
			},
			Spec: v1.PodSpec{
				DNSPolicy: "ClusterFirst",
			},
		}, coptc1)
		coptc2, _ := page.QueryCreateOptions(metav1.CreateOptions{}, "c2")
		_, err = cli.CoreV1().Pods("test").Create(context.Background(), &v1.Pod{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod4",
				Namespace: "test",
			},
			Spec: v1.PodSpec{
				DNSPolicy: "ClusterFirst",
			},
		}, coptc2)
		assert.NoError(t, err)
		lock.Lock()
		defer lock.Unlock()
		assert.Equal(t, []Event{
			{
				Namespace: "test",
				Name:      "pod3",
			},
		}, events)
	})
	t.Run("custom api", func(t *testing.T) {
		_, _ = cli.CoreV1().Pods("test").Create(context.Background(), &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-xxxx-asd",
				Namespace: "test",
				Labels: map[string]string{
					"app": "test",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: "ReplicaSet",
						Name: "test-xxxx",
					},
				},
			},
		}, metav1.CreateOptions{})
		_, _ = cli.CoreV1().Services("test").Create(context.Background(), &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-svc",
				Namespace: "test",
				Labels: map[string]string{
					"app": "test",
				},
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Port: 20880,
					},
				},
				Selector: map[string]string{
					"app": "test",
				},
				ClusterIP: "1.1.1.1",
			},
		}, metav1.CreateOptions{})
		bs, err := cli.Discovery().RESTClient().
			Get().
			RequestURI(fmt.Sprintf("/custom/v1/namespaces/%s/deployments/%s/services",
				// m.Cluster,
				"test",
				"test")).
			DoRaw(context.Background())
		assert.NoError(t, err)
		svcs := make([]v1.Service, 0)
		err = json.Unmarshal(bs, &svcs)
		assert.NoError(t, err)
		assert.Len(t, svcs, 1)
	})
}
