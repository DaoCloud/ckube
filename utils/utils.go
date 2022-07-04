package utils

import "encoding/json"

func Obj2JSONMap(obj interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	bs, _ := json.Marshal(obj)
	_ = json.Unmarshal(bs, &m)
	return m
}
