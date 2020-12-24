# Ckube

Kubernetes APIServer 高性能代理组件，代理 APIServer 的 List 请求，其它类型的请求会直接反向代理到原生 APIServer。
CKube 还额外支持了分页、搜索和索引等功能。
并且，CKube 100% 兼容原生 kubectl 和 kube client sdk，只需要简单的配置即可实现全局替换。

## 安装部署

使用 CKube 构建好的镜像直接启动即可，建议直接部署在 Kubernetes 集群里面，为其绑定 ClusterRole 并赋予权限。
样例 YAML：

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ckube
  labels:
    app: ckube
rules:
- apiGroups: 
    - ""
  resources: 
    - pods
    - services
    - configmaps
    - events
    - namespaces
  verbs:
    - get
    - list
    - watch
    - create
    - update
    - patch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ckube
  labels:
    app: ckube
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ckube
subjects:
- kind: ServiceAccount
  name: ckube
  namespace: kube-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ckube
  namespace: kube-system
  labels:
    app: ckube
---
apiVersion: v1
kind: Service
metadata:
  name: ckube
  labels:
    app: ckube
spec:
  type: ClusterIP
  ports:
    - port: 80
      protocol: TCP
      name: http-ckube
  selector:
    app: ckube

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ckube
  labels:
    app: ckube
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ckube
  template:
    metadata:
      labels:
        app: ckube
    spec:
      dnsPolicy: ClusterFirst
      containers:
        - name: ckube
          image: registry.daocloud.cn/mesh/dx-mesh-ckube:0.0.0-1094
          volumeMounts:
            - readOnly: true
              mountPath: /app/config
              name: ckube-config
      serviceAccountName: ckube
      volumes:
        - name: ckube-config
          configMap:
            name: ckube
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ckube
  labels:
    app: ckube
data:
  local.json: |-
    {
      "proxies": [
        {
          "group": "",
          "version": "v1",
          "resource": "pods",
          "list_kind": "PodList",
          "index": {
            "namespace": "{.metadata.namespace}",
            "name": "{.metadata.name}",
            "labels": "{.metadata.labels}",
            "created_at": "{.metadata.creationTimestamp}"
          }
        },
        {
          "group": "",
          "version": "v1",
          "resource": "services",
          "list_kind": "ServiceList",
          "index": {
            "namespace": "{.metadata.namespace}",
            "name": "{.metadata.name}",
            "labels": "{.metadata.labels}",
            "created_at": "{.metadata.creationTimestamp}"
          }
        },
        {
          "group": "",
          "version": "v1",
          "resource": "namespaces",
          "list_kind": "NamespaceList",
          "index": {
            "namespace": "{.metadata.namespace}",
            "name": "{.metadata.name}",
            "labels": "{.metadata.labels}",
            "created_at": "{.metadata.creationTimestamp}"
          }
        }
      ]
    }

```

## 使用方法

因为 CKube 原生兼容 APIServer 接口，所以，不需要使用额外的 SDK 或切换目前项目中的用法。
如果再程序中需要使用 CKube 来提升性能，或者需要实现分页、搜索等功能，只需要在 SDK 初始化的时候，将地址指定为部署好的 CKube 地址即可。
详细使用方法可以参考 `examples` 目录下的方法。

## 配置方法

参考 `config/example.json` 文件进行配置。
对于每一个需要加速的资源，都需要在配置文件中进行定义，不然无法实现加速和分页等功能。

