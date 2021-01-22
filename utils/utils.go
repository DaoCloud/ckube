package utils

import "encoding/json"

func Obj2JSONMap(obj interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	bs, _ := json.Marshal(obj)
	json.Unmarshal(bs, &m)
	return m
}
