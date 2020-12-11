package store

type GroupVersionResource struct {
	Group    string
	Version  string
	Resource string
}

type QueryResult struct {
	Error error         `json:"error,omitempty"`
	Items []interface{} `json:"items"`
	Total int64         `json:"total"`
}

type Object struct {
	Index map[string]string
	Obj   interface{}
}
