package page

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"gitlab.daocloud.cn/mesh/ckube/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"reflect"
	"strconv"
	"strings"
)

type Paginate struct {
	Page     int64  `json:"page,omitempty"`
	PageSize int64  `json:"page_size,omitempty"`
	Total    int64  `json:"total,omitempty"`
	Sort     string `json:"sort,omitempty"`
	Search   string `json:"search,omitempty"`
}

func (p *Paginate) Match(m map[string]string) (bool, error) {
	parts := p.SearchParts()
	return Match(m, parts)
}

func (p *Paginate) SearchParts() []string {
	search := p.Search
	parts := []string{}
	start := 0
	l := len(search)
	trimPart := func(origin string) string {
		return strings.ReplaceAll(origin, ";;", ";")
	}
	for i := 0; i < l-1; i++ {
		if search[i] == common.SearchPartsSep {
			if search[i+1] == common.SearchPartsSep {
				// double ;, skip
				i += 1
				continue
			} else {
				// need split
				parts = append(parts, trimPart(search[start:i]))
				start = i + 1
			}
		}
	}
	parts = append(parts, search[start:])
	return parts
}

func Match(m map[string]string, searchParts []string) (bool, error) {
	if len(searchParts) != 1 {
		matched := 0
		for _, part := range searchParts {
			r, err := Match(m, []string{part})
			if err != nil {
				return false, err
			}
			if r {
				matched += 1
			}
		}
		return matched == len(searchParts), nil
	}
	search := strings.TrimSpace(searchParts[0])
	if search == "" {
		return true, nil
	}
	if strings.HasPrefix(search, common.AdvancedSearchPrefix) {
		if len(search) == len(common.AdvancedSearchPrefix) {
			return false, fmt.Errorf("search format error")
		}
		selectorStr := search[len(common.AdvancedSearchPrefix):]
		s, err := common.ParseToLabelSelector(selectorStr)
		if err != nil {
			return false, err
		}
		ss, err := v1.LabelSelectorAsSelector(s)
		if err != nil {
			return false, err
		}
		return ss.Matches(labels.Set(m)), nil
	}
	key := ""
	value := ""
	indexOfEqual := strings.Index(search, "=")
	if indexOfEqual < 0 {
		// fuzzy search
		value = search
	} else {
		key = search[:indexOfEqual]
		if indexOfEqual < len(search)-1 {
			value = search[indexOfEqual+1:]
		}
	}
	if key != "" {
		if v, ok := m[key]; !ok {
			return false, fmt.Errorf("unexpected search key: %s", key)
		} else {
			return strings.Contains(strconv.Quote(v), value), nil
		}
	}
	// fuzzy search
	for _, v := range m {
		if strings.Contains(strconv.Quote(v), value) {
			return true, nil
		}
	}
	return false, nil
}

func (p *Paginate) SearchSelector() (*v1.LabelSelector, error) {
	s := v1.LabelSelector{}
	parts := p.SearchParts()
	search := ""
	for _, part := range parts {
		if strings.HasPrefix(part, common.AdvancedSearchPrefix) {
			search = part
			break
		}
	}
	if search == "" {
		return &s, nil
	}
	if !strings.HasPrefix(search, common.AdvancedSearchPrefix) {
		return nil, fmt.Errorf("")
	}
	if len(common.AdvancedSearchPrefix) == len(search) {
		return &s, nil
	}
	return common.ParseToLabelSelector(search[len(common.AdvancedSearchPrefix):])
}

func (p *Paginate) SetSearchSelector(selector *v1.LabelSelector) {
	parts := p.SearchParts()
	pps := []string{common.AdvancedSearchPrefix + v1.FormatLabelSelector(selector)}
	for _, part := range parts {
		if !strings.HasPrefix(part, common.AdvancedSearchPrefix) {
			pps = append(pps, part)
		}
	}
	p.SetSearchWithParts(pps)
}

func (p *Paginate) SetSearchWithParts(parts []string) {
	doubleSep := func(s string) string {
		return strings.ReplaceAll(s, ";", ";;")
	}
	pps := []string{}
	for _, part := range parts {
		pps = append(pps, doubleSep(part))
	}
	p.Search = strings.Join(pps, ";")
}

func (p *Paginate) Namespaces(nss []string) error {
	s, err := p.SearchSelector()
	if err != nil {
		return err
	}
	nsKey := "namespace"
	mes := []v1.LabelSelectorRequirement{
		{
			Key:      nsKey,
			Operator: v1.LabelSelectorOpIn,
			Values:   nss,
		},
	}
	for _, r := range s.MatchExpressions {
		if r.Key != nsKey {
			mes = append(mes, r)
		}
	}
	s.MatchExpressions = mes
	p.SetSearchSelector(s)
	return nil
}

func QueryListOptions(options v1.ListOptions, page Paginate) v1.ListOptions {
	bs, _ := json.Marshal(page)
	if string(bs) == "{}" {
		return options
	}
	s := base64.StdEncoding.WithPadding(base64.NoPadding).EncodeToString(bs)
	if options.LabelSelector == "" {
		options.LabelSelector = fmt.Sprintf("%s notin (%s)", common.PaginateKey, s)
		return options
	}
	ls, err := common.ParseToLabelSelector(options.LabelSelector)
	if err != nil {
		return options
	}
	mes := []v1.LabelSelectorRequirement{{
		Key:      common.PaginateKey,
		Operator: v1.LabelSelectorOpNotIn,
		Values:   []string{s},
	}}
	for _, m := range ls.MatchExpressions {
		if m.Key != common.PaginateKey {
			mes = append(mes, m)
		}
	}
	ls.MatchExpressions = mes
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
