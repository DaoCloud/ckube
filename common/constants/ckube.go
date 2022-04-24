package constants

const (
	PaginateKey          = "ckube.daocloud.io/query"
	AdvancedSearchPrefix = "__ckube_as__:"
	SortASC              = "asc"
	SortDesc             = "desc"
	KeyTypeSep           = "!"
	KeyTypeInt           = "int"
	KeyTypeStr           = "str"
	SearchPartsSep       = ';'
	DSMClusterAnno       = "ckube.doacloud.io/cluster"
	ClusterPrefix        = "dsm-cluster-"
	IndexAnno            = "ckube.daocloud.io/indexes"
)

var (
	_ = PaginateKey
	_ = AdvancedSearchPrefix
	_ = SortASC
	_ = SortDesc
	_ = KeyTypeSep
	_ = KeyTypeInt
	_ = KeyTypeStr
	_ = SearchPartsSep
	_ = DSMClusterAnno
	_ = ClusterPrefix
	_ = IndexAnno
)
