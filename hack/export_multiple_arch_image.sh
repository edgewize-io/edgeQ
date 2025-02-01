#!/bin/bash

set -e
set -o pipefail

REPO=${REPO:-harbor.dev.thingsdao.com}
PROJECT=${PROJECT:-edgewize}
TAG=${TAG:-v0.1.3}

#PLATFORMS=linux/amd64,linux/arm64
PLATFORMS=("amd64" "arm64")
components=("model-mesh-proxy" "model-mesh-broker" "model-mesh-msc")

for component in "${components[@]}";
do
  for platform in "${PLATFORMS[@]}";
  do
    echo "exporting ${platform} Architecture ${REPO}/${PROJECT}/${component}:${TAG}"
    docker pull --platform linux/${platform} ${REPO}/${PROJECT}/${component}:${TAG}
    docker inspect ${REPO}/${PROJECT}/${component}:${TAG}|grep Architecture|grep -q ${platform}
    if [ $? -ne 0 ]; then
        echo "image Architecture is incorrect, exit"
        exit
    fi

    docker tag ${REPO}/${PROJECT}/${component}:${TAG} ${REPO}/${PROJECT}/${component}:${TAG}-${platform}
    docker save ${REPO}/${PROJECT}/${component}:${TAG}-${platform} | gzip > ${REPO}_${PROJECT}_${component}_${TAG}-${platform}.tgz
    echo "export image file ${REPO}_${PROJECT}_${component}_${TAG}-${platform}.tgz successfully"
  done

done
