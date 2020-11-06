package common

type Proxy struct {
	Group    string            `json:"group"`
	Version  string            `json:"version"`
	Resource string            `json:"resource"`
	ListKind string            `json:"list_kind"`
	Index    map[string]string `json:"index"`
}

type Config struct {
	Proxies []Proxy `json:"proxies"`
}
