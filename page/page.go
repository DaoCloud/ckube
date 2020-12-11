package page

import (
	"encoding/base64"
	"encoding/json"
	"gitlab.daocloud.cn/mesh/ckube/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Paginate struct {
	Page     int64  `json:"page,omitempty"`
	PageSize int64  `json:"page_size,omitempty"`
	Total    int64  `json:"total,omitempty"`
	Reverse  bool   `json:"reverse,omitempty"`
	Sort     string `json:"sort,omitempty"`
	Search   string `json:"search,omitempty"`
}

func QueryListOptions(options v1.ListOptions, page Paginate) v1.ListOptions {
	bs, _ := json.Marshal(page)
	if string(bs) == "{}" {
		return options
	}
	s := base64.StdEncoding.WithPadding(base64.NoPadding).EncodeToString(bs)
	if options.LabelSelector == "" {
		options.LabelSelector = common.PaginateKey + "=" + s
		return options
	}
	ls, err := common.ParseToLabelSelector(options.LabelSelector)
	if err != nil {
		return options
	}
	ls.MatchLabels[common.PaginateKey] = s
	options.LabelSelector = v1.FormatLabelSelector(ls)

	return options
}
