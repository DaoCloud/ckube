package kube

import (
	"fmt"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8labels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sort"
	"strings"
)

const eachPartLen = 56

func SplittingValue(value string) []string {
	res := []string{}
	if len(value) <= eachPartLen {
		return []string{value}
	}
	for i := 0; ; i += eachPartLen {
		end := i + eachPartLen
		if len(value) <= end {
			res = append(res, fmt.Sprintf("%04d.%s", i, value[i:]))
			break
		}
		res = append(res, fmt.Sprintf("%04d.%s", i, value[i:end]))
	}
	return res
}

func MergeValues(values []string) (string, error) {
	if len(values) == 1 {
		return values[0], nil
	}
	sort.Strings(values)
	b := strings.Builder{}
	for _, v := range values {
		parts := strings.Split(v, ".")
		if len(parts) != 2 {
			return "", fmt.Errorf("value format error")
		}
		b.WriteString(parts[1])
	}
	return b.String(), nil
}

func ParseToLabelSelector(selector string) (*v1.LabelSelector, error) {
	reqs, err := k8labels.ParseToRequirements(selector)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse the selector string \"%s\": %v", selector, err)
	}

	labelSelector := &v1.LabelSelector{
		MatchLabels:      map[string]string{},
		MatchExpressions: []v1.LabelSelectorRequirement{},
	}
	for _, req := range reqs {
		var op v1.LabelSelectorOperator
		switch req.Operator() {
		case selection.Equals, selection.DoubleEquals:
			vals := req.Values()
			if vals.Len() != 1 {
				return nil, fmt.Errorf("equals operator must have exactly one value")
			}
			val, ok := vals.PopAny()
			if !ok {
				return nil, fmt.Errorf("equals operator has exactly one value but it cannot be retrieved")
			}
			labelSelector.MatchLabels[req.Key()] = val
			continue
		case selection.In:
			op = v1.LabelSelectorOpIn
		case selection.NotIn, selection.NotEquals:
			op = v1.LabelSelectorOpNotIn
		case selection.Exists:
			op = v1.LabelSelectorOpExists
		case selection.DoesNotExist:
			op = v1.LabelSelectorOpDoesNotExist
		case selection.GreaterThan, selection.LessThan:
			// Adding a separate case for these operators to indicate that this is deliberate
			return nil, fmt.Errorf("%q isn't supported in label selectors", req.Operator())
		default:
			return nil, fmt.Errorf("%q is not a valid label selector operator", req.Operator())
		}
		labelSelector.MatchExpressions = append(labelSelector.MatchExpressions, v1.LabelSelectorRequirement{
			Key:      req.Key(),
			Operator: op,
			Values:   req.Values().List(),
		})
	}
	return labelSelector, nil
}
