package store

type GroupVersionResource struct {
	Group    string
	Version  string
	Resource string
}

type Paginate struct {
	Page     int64  `json:"page"`
	PageSize int64  `json:"page_size"`
	Total    int64  `json:"total"`
	Reverse  bool   `json:"reverse"`
	Sort     string `json:"sort"`
	Search   string `json:"search"`
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
