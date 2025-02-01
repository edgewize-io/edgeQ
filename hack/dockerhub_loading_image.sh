#!/bin/bash

set -e
set -o pipefail

OLD_REPO=${OLD_REPO:-harbor.dev.thingsdao.com}
NEW_REPO=${NEW_REPO:-dockerhub.kubekey.local}
PROJECT=${PROJECT:-edgewize}
OLD_TAG=${OLD_TAG:-v0.1.3}
NEW_TAG=${NEW_TAG:-v0.1.3}


#PLATFORMS=linux/amd64,linux/arm64
PLATFORMS=("amd64" "arm64")
components=("model-mesh-proxy" "model-mesh-broker")


for component in "${components[@]}";
do
  IMAGE_LIST=" "

  for platform in "${PLATFORMS[@]}";
  do
    if [ -e ${OLD_REPO}_${PROJECT}_${component}_${OLD_TAG}-${platform}.tgz ]; then
      echo "loading ${OLD_REPO}_${PROJECT}_${component}_${OLD_TAG}-${platform}.tgz"
    else
      echo "image file ${OLD_REPO}_${PROJECT}_${component}_${OLD_TAG}-${platform}.tgz not found"
      exit
    fi

    docker load -i ${OLD_REPO}_${PROJECT}_${component}_${OLD_TAG}-${platform}.tgz
    docker tag ${OLD_REPO}/${PROJECT}/${component}:${OLD_TAG}-${platform} ${NEW_REPO}/${PROJECT}/${component}:${NEW_TAG}-${platform}
    echo "load image file ${OLD_REPO}_${PROJECT}_${component}_${OLD_TAG}-${platform}.tgz successfully"
    docker push ${NEW_REPO}/${PROJECT}/${component}:${NEW_TAG}-${platform}
    echo "tag image to ${NEW_REPO}/${PROJECT}/${component}:${NEW_TAG}-${platform} and push to ${NEW_REPO}"
    IMAGE_LIST="${IMAGE_LIST} ${NEW_REPO}/${PROJECT}/${component}:${NEW_TAG}-${platform}"
  done

  docker manifest create ${NEW_REPO}/${PROJECT}/${component}:${NEW_TAG} ${IMAGE_LIST}
  docker manifest push ${NEW_REPO}/${PROJECT}/${component}:${NEW_TAG}
  echo "create manifest ${NEW_REPO}/${PROJECT}/${component}:${NEW_TAG} successfully"
done
