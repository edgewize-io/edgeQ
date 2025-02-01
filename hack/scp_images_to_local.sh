#!/bin/bash

set -e
set -o pipefail

REPO=${REPO:-harbor.dev.thingsdao.com}
PROJECT=${PROJECT:-edgewize}
OLD_TAG=${OLD_TAG:-v0.1.3}


#PLATFORMS=linux/amd64,linux/arm64
PLATFORMS=("amd64" "arm64")
components=("model-mesh-proxy" "model-mesh-broker" "model-mesh-msc")

for component in "${components[@]}";
do
  for platform in "${PLATFORMS[@]}";
  do
    scp root@172.31.53.54:/root/${REPO}_${PROJECT}_${component}_${OLD_TAG}-${platform}.tgz ./
    echo "下载镜像 ${REPO}_${PROJECT}_${component}_${OLD_TAG}-${platform}.tgz 成功"
  done
done