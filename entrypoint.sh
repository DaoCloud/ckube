#!/bin/sh

set -e

token=$(cat /run/secrets/kubernetes.io/serviceaccount/token)
server=${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT_HTTPS}

sed -i "s/__KUBE_TOKEN__/${token}/g" /etc/nginx/nginx.conf
sed -i "s/__KUBE_SERVER__/${server}/g" /etc/nginx/nginx.conf

# start nginx
nginx

/app/dist/cacheproxy $@
