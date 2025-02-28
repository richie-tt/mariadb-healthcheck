#!/bin/bash

echo "Starting the release tagging process"

function getMajorVersion() {
  local BRANCH=$(git branch --show-current)
  if [[ ${BRANCH} == *"release"* ]]; then
    echo $(echo ${BRANCH} | grep -o -E 'v[0-9]+')
  else
    echo wrong_branch
  fi
}

function getLatestTag() {
  echo $(git --no-pager tag -l "${MAJOR}.*" --sort="-version:refname" | head -n1)
}

MAJOR=$(getMajorVersion)

if [[ ${MAJOR} == "wrong_branch" ]]; then
  echo "This is not release/v* branch. Nothing to do"
  exit 1
fi

# get highest tag in specific release/v* branch
LATEST_TAG=$(getLatestTag)

if [[ ${LATEST_TAG} == '' ]]; then
  VERSION="${MAJOR}.0.0"
else
  VERSION=${LATEST_TAG}
fi

echo "Latest version tag: ${VERSION}"

# split version tag into array
VERSION_BITS=(${VERSION//./ })

#get number parts and increase last one by 1
VNUM1=${VERSION_BITS[0]}
VNUM2=${VERSION_BITS[1]}
VNUM3=${VERSION_BITS[2]}

COUNT_OF_COMMIT_MSG_HAVE_SEMVER_MINOR=$(git log -1 --pretty=%B | egrep -ci '(feature)|(feat)|(minor)')
TO_PUSH=false

if [ ${COUNT_OF_COMMIT_MSG_HAVE_SEMVER_MINOR} -gt 0 ]; then
    VNUM2=$((VNUM2+1))
    VNUM3=0
    TO_PUSH=true
else
    VNUM3=$((VNUM3+1))
    TO_PUSH=true
fi

#create new tag
NEW_TAG="${VNUM1}.${VNUM2}.${VNUM3}"

echo "Updating ${VERSION} to ${NEW_TAG}"

#get current hash and see if it already has a tag
GIT_COMMIT=$(git rev-parse HEAD)
NEEDS_TAG=$(git describe --contains ${GIT_COMMIT} 2>/dev/null)

#only tag if commit message have version-bump-message as mentioned above
if [ -z "${NEEDS_TAG}" ]; then
    if [ ${TO_PUSH} ]; then
        echo "Login with gh-cli"
        gh auth login --hostname github.com
        echo "Tagged with ${NEW_TAG}"
        gh release create ${NEW_TAG} --generate-notes --target ${GIT_COMMIT}
        echo "Success"
    else
        echo "Failed"
        exit 1
    fi
else
    echo "Already a tag on this commit"
fi
