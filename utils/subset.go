package utils

func IsSubsetOf(sub map[string]string, parent map[string]string) bool {
	for k, v := range sub {
		if vv, ok := parent[k]; !ok || vv != v {
			return false
		}
	}
	return true
}
