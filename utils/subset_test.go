package utils

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsSubsetOf(t *testing.T) {
	cases := []struct {
		name   string
		sub    map[string]string
		parent map[string]string
		expect bool
	}{
		{
			"normal",
			map[string]string{
				"a": "b",
			},
			map[string]string{
				"a": "b",
				"b": "c",
			},
			true,
		},
		{
			"multi sub",
			map[string]string{
				"a": "b",
				"b": "c",
			},
			map[string]string{
				"a": "b",
				"b": "c",
				"c": "d",
			},
			true,
		},
		{
			"not sub",
			map[string]string{
				"a": "c",
			},
			map[string]string{
				"a": "b",
				"b": "c",
			},
			false,
		},
		{
			"not sub 2",
			map[string]string{
				"a": "c",
			},
			map[string]string{
				"b": "c",
			},
			false,
		},
		{
			"nil sub",
			nil,
			map[string]string{
				"a": "b",
				"b": "c",
			},
			true,
		},
		{
			"nil parent",
			map[string]string{
				"a": "c",
			},
			nil,
			false,
		},
		{
			"both nil",
			nil,
			nil,
			true,
		},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("%d---%s", i, c.name), func(t *testing.T) {
			res := IsSubsetOf(c.sub, c.parent)
			assert.Equal(t, c.expect, res)
		})
	}
}
