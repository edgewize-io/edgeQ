#!/bin/bash

set -ex
set -o pipefail

OLD_REPO=${OLD_REPO:-harbor.dev.thingsdao.com}
NEW_REPO=${NEW_REPO:-harbor.dev.thingsdao.com}
PROJECT=${PROJECT:-edgewize}
OLD_TAG=${OLD_TAG:-v0.1.3}
NEW_TAG=${NEW_TAG:-v0.1.3}


#PLATFORMS=linux/amd64,linux/arm64
PLATFORMS=("amd64" "arm64")
components=("model-mesh-proxy" "model-mesh-broker" "model-mesh-msc")


for component in "${components[@]}";
do
  IMAGE_LIST=" "

  for platform in "${PLATFORMS[@]}";
  do
    if [ -e ${OLD_REPO}_${PROJECT}_${component}_${OLD_TAG}-${platform}.tgz ]; then
      echo "正在加载镜像包 ${OLD_REPO}_${PROJECT}_${component}_${OLD_TAG}-${platform}.tgz"
    else
      echo "镜像包 ${OLD_REPO}_${PROJECT}_${component}_${OLD_TAG}-${platform}.tgz 未找到"
      exit
    fi

    docker load -i ${OLD_REPO}_${PROJECT}_${component}_${OLD_TAG}-${platform}.tgz
    docker tag ${OLD_REPO}/${PROJECT}/${component}:${OLD_TAG}-${platform} ${NEW_REPO}/${PROJECT}/${component}:${NEW_TAG}-${platform}
    echo "加载镜像 ${OLD_REPO}_${PROJECT}_${component}_${OLD_TAG}-${platform}.tgz 成功"
    docker push ${NEW_REPO}/${PROJECT}/${component}:${NEW_TAG}-${platform}
    echo "重新打 tag 为 ${NEW_REPO}/${PROJECT}/${component}:${NEW_TAG}-${platform} 并推送到新仓库"
    IMAGE_LIST="${IMAGE_LIST} ${NEW_REPO}/${PROJECT}/${component}:${NEW_TAG}-${platform}"
  done

  docker manifest create ${NEW_REPO}/${PROJECT}/${component}:${NEW_TAG} ${IMAGE_LIST}
  docker manifest push ${NEW_REPO}/${PROJECT}/${component}:${NEW_TAG}
  echo "创建 manifest ${NEW_REPO}/${PROJECT}/${component}:${NEW_TAG} 成功"
done
