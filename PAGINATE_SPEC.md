# Paginate Spec

本文描述 CKube 的查询、分页功能定义。

## Page 
Page 表示页码，从 1 开始，为 0 表示不分页。小于 0  为非法。

## PageSize
PageSize 表示每页数量，为 0  表示不分页，小于 0 为非法。

## Search

### Search DSL
CKube 的搜索支持模糊搜索（Fuzzy Search）,精准匹配和高级搜索。
如果同时有模糊搜索、精准字段匹配或高级搜索的多个，以 `;` 分割，如果需要搜索 `;`，请使用两个： `;;`。
如 `name=e; __ckube_as__:namespaces in (default, test)`

#### 模糊搜索 Fuzzy Search
模糊搜索的句式为任意字符串，但是中间不能包含 `=` 和 `!`.
模糊搜索的算法为，只要`任意索引中的值包含`此项，即为匹配，会返回此结果。
以 `!` 开头，表示不匹配。

#### 精准字段匹配
精准匹配的句式为 `key=value`，其中，`key` 为索引的 Key，`value` 为需要匹配的内容。
`key` 需要满足 `[\d\w\-_\.]` 的规则，`value` 可以是任意值。
精准匹配的算法为，直接对索引的键值做匹配，只要索引中的值包含 `value` 即为匹配成功，如果要精准匹配，可将 `value` 加上双引号，如 `name="test"`。`value` 以 `!` 开头表示不匹配。

#### 高级搜索
主要针对需要同时对多个字段进行匹配，或者需要执行 `in`, `notin`, `!=` 等操作。
高级搜索的句式为 `__ckube_as__:<LABEL_SELECTOR>`, `LABEL_SELECTOR` 的语法参考：https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
大致总结如下：

| 需求 | 样例 | 
| -- | -- |
| 精确匹配某个字段 | `__ckube_as__:name=ok` |
| 不匹配 | `__ckube_as__:name!=ok` |
| 匹配列表 | `__ckube_as__:name in (a,b,c)` |
| 不匹配列表 | `__ckube_as__:name notin (a,b,c)` |
| 多个条件 | `__ckube_as__:name=ok,sp notin (ok, 1)` |

高级搜索会按照规则对语句进行解析，然后逐个匹配。

## Sort

Sort 用于对结果进行排序，CKube 支持同时对多个字段进行排序，并且支持`字符串`和`数字`类型的字段进行排序。

### Sort DSL

搜索的句式为 `key[!int|!str][ desc|asc][,key[!int|!str][ desc|asc]...]`.
`[]` 表示可选。
`!int` 如果存在，表示对字段进行强制数字转换，默认使用字符串排序规则进行排序。
`desc` 表示使用该字段进行反向排序，`asc` 表示对该字段进行正向排序。
如果排序字段为空，将使用 uid,name 优先的字段进行排序。

#### 样例
按照命名空间，名字进行排序 `namespace,name`.
按照命名空间反序，副本数进行排序 `namespace desc,replicas!int`.
按照创建时间进行排序 `createTimestamp!int desc`.
