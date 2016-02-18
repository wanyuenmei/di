#!/bin/bash

# Exit upon any error
set -e

if [ "$TRAVIS_PULL_REQUEST" != "false" ]; then
    echo "This is a pull request, not building minion."
    exit 0
fi

if [ "$TRAVIS_BRANCH" != "master" ]; then
    echo "This is not the master branch, not building minion."
    exit 0
fi

echo "Building minion..."

make docker
echo "Successfully built minion."

docker login -e="$DOCKER_EMAIL" -u="$DOCKER_USERNAME" -p="$DOCKER_PASSWORD"
echo "Successfully logged into docker."

docker push quay.io/netsys/di-minion
echo "Successfully pushed minion."
