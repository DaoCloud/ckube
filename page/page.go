package page

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/DaoCloud/ckube/common/constants"
	"github.com/DaoCloud/ckube/kube"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type Paginate struct {
	Page     int64  `json:"page,omitempty" form:"page"`
	PageSize int64  `json:"page_size,omitempty" form:"page_size"`
	Total    int64  `json:"total,omitempty"form:"total" `
	Sort     string `json:"sort,omitempty" form:"sort"`
	Search   string `json:"search,omitempty" form:"search"`
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
		if search[i] == constants.SearchPartsSep {
			if search[i+1] == constants.SearchPartsSep {
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
	parts = append(parts, trimPart(search[start:]))
	ps := []string{}
	for _, p := range parts {
		if p != "" {
			ps = append(ps, p)
		}
	}
	return ps
}

func parseValue(v string) (string, bool) {
	if strings.HasPrefix(v, "!") {
		return v[1:], true
	}
	return v, false
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
	if strings.HasPrefix(search, constants.AdvancedSearchPrefix) {
		if len(search) == len(constants.AdvancedSearchPrefix) {
			return false, fmt.Errorf("search format error")
		}
		selectorStr := search[len(constants.AdvancedSearchPrefix):]
		s, err := kube.ParseToLabelSelector(selectorStr)
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
	value, reverse := parseValue(value)
	if key != "" {
		if v, ok := m[key]; !ok {
			return false, fmt.Errorf("unexpected search key: %s", key)
		} else {
			vv := strings.Contains(strconv.Quote(v), value)
			if reverse {
				return !vv, nil
			}
			return vv, nil
		}
	}
	// fuzzy search
	for _, v := range m {
		vv := strings.Contains(strconv.Quote(v), value)
		if reverse {
			if vv {
				return false, nil
			}
		} else {
			if vv {
				return true, nil
			}
		}
	}
	return reverse, nil
}

func (p *Paginate) SearchSelector() (*v1.LabelSelector, error) {
	s := v1.LabelSelector{}
	parts := p.SearchParts()
	search := ""
	for _, part := range parts {
		if strings.HasPrefix(part, constants.AdvancedSearchPrefix) {
			search = part
			break
		}
	}
	if search == "" {
		return &s, nil
	}
	if !strings.HasPrefix(search, constants.AdvancedSearchPrefix) {
		return nil, fmt.Errorf("")
	}
	if len(constants.AdvancedSearchPrefix) == len(search) {
		return &s, nil
	}
	return kube.ParseToLabelSelector(search[len(constants.AdvancedSearchPrefix):])
}

func (p *Paginate) SetSearchSelector(selector *v1.LabelSelector) error {
	parts := p.SearchParts()
	sstr := v1.FormatLabelSelector(selector)
	if sstr == "<error>" {
		return fmt.Errorf("parse selector %v error", selector)
	}
	pps := []string{constants.AdvancedSearchPrefix + sstr}
	for _, part := range parts {
		if !strings.HasPrefix(part, constants.AdvancedSearchPrefix) {
			pps = append(pps, part)
		}
	}
	p.SetSearchWithParts(pps)
	return nil
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
	return p.SetSearchSelector(s)
}

func (p *Paginate) Clusters(css []string) error {
	s, err := p.SearchSelector()
	if err != nil {
		return err
	}
	for _, c := range css {
		if c == "" {
			return fmt.Errorf("unexpected cluster %q", c)
		}
	}
	nsKey := "cluster"
	mes := []v1.LabelSelectorRequirement{
		{
			Key:      nsKey,
			Operator: v1.LabelSelectorOpIn,
			Values:   css,
		},
	}
	for _, r := range s.MatchExpressions {
		if r.Key != nsKey {
			mes = append(mes, r)
		}
	}
	s.MatchExpressions = mes
	return p.SetSearchSelector(s)
}

func (p *Paginate) GetClusters() []string {
	if p == nil {
		return nil
	}
	s, err := p.SearchSelector()
	if err != nil {
		return nil
	}
	nsKey := "cluster"
	for _, r := range s.MatchExpressions {
		if r.Key == nsKey {
			return r.Values
		}
	}
	return nil
}

func QueryGetOptions(options v1.GetOptions, cluster string) (v1.GetOptions, error) {
	if options.ResourceVersion != "" {
		return options, fmt.Errorf("can not set ResourceVersion if wrap cluster for GetOptions")
	}
	options.ResourceVersion = "dsm-cluster-" + cluster
	return options, nil
}

func QueryCreateOptions(options v1.CreateOptions, cluster string) (v1.CreateOptions, error) {
	if options.FieldManager != "" {
		return options, fmt.Errorf("can not set ResourceVersion if wrap cluster for CreateOptions")
	}
	options.FieldManager = "dsm-cluster-" + cluster
	return options, nil
}

func QueryUpdateOptions(options v1.UpdateOptions, cluster string) (v1.UpdateOptions, error) {
	if options.FieldManager != "" {
		return options, fmt.Errorf("can not set ResourceVersion if wrap cluster for UpdateOptions")
	}
	options.FieldManager = "dsm-cluster-" + cluster
	return options, nil
}

func QueryPatchOptions(options v1.PatchOptions, cluster string) (v1.PatchOptions, error) {
	if len(options.FieldManager) != 0 {
		return options, fmt.Errorf("can not set ResourceVersion if wrap cluster for PatchOptions")
	}
	options.FieldManager = "dsm-cluster-" + cluster
	return options, nil
}

func QueryDeleteOptions(options v1.DeleteOptions, cluster string) (v1.DeleteOptions, error) {
	if len(options.DryRun) != 0 {
		return options, fmt.Errorf("can not set ResourceVersion if wrap cluster for DeleteOptions")
	}
	options.DryRun = []string{"dsm-cluster-" + cluster}
	return options, nil
}

func QueryListOptions(options v1.ListOptions, page Paginate) (v1.ListOptions, error) {
	bs, _ := json.Marshal(page)
	if string(bs) == "{}" {
		return options, nil
	}
	s := base64.StdEncoding.WithPadding(base64.NoPadding).EncodeToString(bs)
	//if options.LabelSelector == "" {
	//	options.LabelSelector = fmt.Sprintf("%s notin (%s)", common.PaginateKey, s)
	//	return options
	//}
	ls, err := kube.ParseToLabelSelector(options.LabelSelector)
	if err != nil {
		return options, err
	}
	mes := []v1.LabelSelectorRequirement{{
		Key:      constants.PaginateKey,
		Operator: v1.LabelSelectorOpNotIn,
		Values:   kube.SplittingValue(s),
	}}
	for _, m := range ls.MatchExpressions {
		if m.Key != constants.PaginateKey {
			mes = append(mes, m)
		}
	}
	ls.MatchExpressions = mes
	sstr := v1.FormatLabelSelector(ls)
	if sstr == "<error>" {
		return options, fmt.Errorf("parse selector %v error", ls)
	}
	options.LabelSelector = sstr
	return options, nil
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

func GetObjectCluster(o v1.Object) string {
	return o.GetAnnotations()[constants.DSMClusterAnno]
}
