#!/bin/bash

function status_line() {
    echo -e "\n### ${1} ###\n"
}

# Exit upon any error
set -e

status_line "Begin build..."

make all check lint

if [[ $(make -s lint 2>&1) ]] ; then # golint doesn't fail, just prints things.
    exit 1
fi

status_line "Building minion..."

make docker
status_line "Successfully built minion."

if [ "$TRAVIS_PULL_REQUEST" != "false" ]; then
    status_line "This is a pull request, not pushing minion."
    exit 0
fi

if [ "$TRAVIS_BRANCH" != "master" ]; then
    status_line "This is not the master branch, not pushing minion."
    exit 0
fi

docker version

status_line "Pushing minion..."

docker login -e="$DOCKER_EMAIL" -u="$DOCKER_USERNAME" -p="$DOCKER_PASSWORD" quay.io
status_line "Successfully logged into docker."

docker push quay.io/netsys/di
status_line "Successfully pushed di."

docker push quay.io/netsys/di-minion
status_line "Successfully pushed minion."
