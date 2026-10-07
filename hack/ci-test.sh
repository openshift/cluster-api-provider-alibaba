#!/bin/bash

# Copyright 2021 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

cd "$REPO_ROOT"
# shellcheck source=hack/fetch-ext-bins.sh
source ./hack/fetch-ext-bins.sh

function cleanup_ci_envtest {
	local test_status=$?
	trap - EXIT

	if ! cleanup_envtest_tools && [ "$test_status" -eq 0 ]; then
		test_status=1
	fi
	exit "$test_status"
}

# fetch-ext-bins.sh is sourced and deliberately does not alter caller traps.
# This caller owns the fetched assets until the unit tests have finished.
trap cleanup_ci_envtest EXIT

fetch_tools
setup_envs
make unit
