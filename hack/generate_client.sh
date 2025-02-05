#!/bin/bash

set -e

GV="$1"

rm -rf ./pkg/client
./hack/generate_group.sh "client,lister,informer" pkg/client github.com/edgewize/edgeQ/pkg/apis "${GV}" --output-base=./  -h "$PWD/hack/boilerplate.go.txt"
