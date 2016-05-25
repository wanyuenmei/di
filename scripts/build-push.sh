#!/bin/bash

function status_line() {
    echo -e "\n### ${1} ###\n"
}

# Exit upon any error
set -e
set -x

status_line "Begin build..."

make all check lint

if [[ $(make -s lint 2>&1) ]] ; then # golint doesn't fail, just prints things.
    exit 1
fi

status_line "Building containers..."

make docker-build-quilt docker-build-tester docker-build-minion
status_line "Successfully built containers."

if [ "$TRAVIS_PULL_REQUEST" != "false" ]; then
    status_line "This is a pull request, not pushing containers."
    exit 0
fi

if [ "$TRAVIS_BRANCH" != "master" ]; then
    status_line "This is not the master branch, not pushing containers."
    exit 0
fi

docker version

status_line "Pushing containers..."

docker login -e="$DOCKER_EMAIL" -u="$DOCKER_USERNAME" -p="$DOCKER_PASSWORD" docker.io
status_line "Successfully logged into docker."

make docker-push-quilt docker-push-tester docker-push-minion
status_line "Successfully pushed containers."
