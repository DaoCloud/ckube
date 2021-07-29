package common

type Proxy struct {
	Group    string            `json:"group"`
	Version  string            `json:"version"`
	Resource string            `json:"resource"`
	ListKind string            `json:"list_kind"`
	Index    map[string]string `json:"index"`
}

type Cluster struct {
	Context string `json:"context"`
}

type Config struct {
	Proxies        []Proxy            `json:"proxies"`
	Clusters       map[string]Cluster `json:"clusters"`
	DefaultCluster string             `json:"default_cluster"`
}

var cfg *Config

func InitConfig(c *Config) {
	cfg = c
}

func GetConfig() Config {
	return *cfg
}

func GetGVRKind(g, v, r string) string {
	for _, p := range cfg.Proxies {
		if p.Group == g && p.Version == v && p.Resource == r {
			return p.ListKind
		}
	}
	return ""
}
