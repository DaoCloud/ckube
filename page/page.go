package page

import (
	"encoding/base64"
	"encoding/json"
	"gitlab.daocloud.cn/mesh/ckube/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
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
		options.LabelSelector = common.PaginateKey + "!=" + s
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

func MakeupResPaginate(l v1.ListInterface, page Paginate) Paginate {
	remain := l.GetRemainingItemCount()
	val := reflect.ValueOf(l).Elem()
	items := 0
	for i := 0; i < val.NumField(); i++ {
		valueField := val.Field(i)
		typeField := val.Type().Field(i)

		f := valueField.Interface()
		val := reflect.ValueOf(f)
		if typeField.Name == "Items" {
			items = val.Len()
		}
	}
	if remain == nil {
		var i int64 = 0
		remain = &i
	}
	page.Total = *remain + (page.Page-1)*page.PageSize + int64(items)
	return page
}
