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
