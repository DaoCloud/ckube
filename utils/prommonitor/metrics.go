package prommonitor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gitlab.daocloud.cn/mesh/ckube/api"
)

const (
	ComponentMetricsLabel string = "component"
	CkubeComponent        string = "ckube"
)

var (
	Up = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "up",
		Help: "Component up status",
	}, []string{ComponentMetricsLabel})
	ConfigReload = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ckube_reload_config_total",
		Help: "Config reload count",
	}, []string{"status"})
	Requests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ckube_requests_total",
		Help: "Requests count",
	}, []string{"cluster", "group", "version", "kind", "single", "cached"})
)

func PromHandler(r *api.ReqContext) interface{} {
	promhttp.Handler().ServeHTTP(r.Writer, r.Request)

	return nil
}
