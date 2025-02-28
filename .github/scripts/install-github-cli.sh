#!/bin/bash

. ${BASE_DIR}/.github/scripts/.versions

curl -o gh.tar.gz -sSL "https://github.com/cli/cli/releases/download/v${GH_CLI_VERSION}/gh_${GH_CLI_VERSION}_linux_amd64.tar.gz"
tar xvf gh.tar.gz "gh_${GH_CLI_VERSION}_linux_amd64/bin/gh"
sudo install -o root -g root -m 755 "gh_${GH_CLI_VERSION}_linux_amd64/bin/gh" /usr/bin/
