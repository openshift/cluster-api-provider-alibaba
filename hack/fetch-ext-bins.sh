#!/usr/bin/env bash
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

# Enable tracing in this script off by setting the TRACE variable in your
# environment to any value:
#
# $ TRACE=1 test.sh
TRACE=${TRACE:-""}
if [ -n "$TRACE" ]; then
  set -x
fi

k8s_version=1.23.5
goarch=amd64
goos="unknown"

if [[ "$OSTYPE" == "linux"* ]]; then
  goos="linux"
elif [[ "$OSTYPE" == "darwin"* ]]; then
  goos="darwin"
fi

if [[ "$goos" == "unknown" ]]; then
  echo "OS '$OSTYPE' not supported. Aborting." >&2
  exit 1
fi

# Turn colors in this script off by setting the NO_COLOR variable in your
# environment to any value:
#
# $ NO_COLOR=1 test.sh
NO_COLOR=${NO_COLOR:-""}
if [ -z "$NO_COLOR" ]; then
  header=$'\e[1;33m'
  reset=$'\e[0m'
else
  header=''
  reset=''
fi

function header_text {
  echo "$header$*$reset"
}

tmp_root=${ENVTEST_TMP_ROOT:-/tmp}
envtest_bin_dir=$tmp_root/controller-tools/envtest
envtest_cleanup_dir=''

# Skip fetching and untaring the tools by setting the SKIP_FETCH_TOOLS variable
# in your environment to any value:
#
# $ SKIP_FETCH_TOOLS=1 ./fetch_ext_bins.sh
#
# If you skip fetching tools, this script will use the tools already installed
# in envtest_bin_dir.
SKIP_FETCH_TOOLS=${SKIP_FETCH_TOOLS:-""}

function validate_envtest_assets {
  local asset_dir=$1
  local binary

  for binary in kubectl kube-apiserver etcd; do
    if [ ! -f "$asset_dir/$binary" ] || [ ! -x "$asset_dir/$binary" ]; then
      echo "envtest asset $asset_dir/$binary is not a regular executable file" >&2
      return 1
    fi
  done
}

function validate_fetched_envtest_assets {
  local asset_dir=$1
  local controller_tools_dir=${asset_dir%/*}
  local directory
  local binary

  for directory in "$controller_tools_dir" "$asset_dir"; do
    if [ ! -d "$directory" ] || [ -L "$directory" ]; then
      echo "fetched envtest asset directory $directory must be a real directory" >&2
      return 1
    fi
  done
  if ! validate_envtest_assets "$asset_dir"; then
    return 1
  fi
  for binary in kubectl kube-apiserver etcd; do
    if [ -L "$asset_dir/$binary" ]; then
      echo "fetched envtest asset $asset_dir/$binary must not be a symlink" >&2
      return 1
    fi
  done
}

function cleanup_envtest_tools {
  if [ -z "$envtest_cleanup_dir" ]; then
    return 0
  fi

  local cleanup_dir=$envtest_cleanup_dir
  envtest_cleanup_dir=''
  if ! rm -rf "$cleanup_dir"; then
    echo "failed to remove envtest extraction directory $cleanup_dir" >&2
    return 1
  fi
}

function extract_envtest_archive {
  local archive_path=$1
  local extraction_dir=$2

  if ! tar -zxf "$archive_path" -C "$extraction_dir"; then
    echo "failed to extract envtest tools from $archive_path" >&2
    return 1
  fi
  validate_fetched_envtest_assets "$extraction_dir/controller-tools/envtest"
}

# Fetch the Kubernetes API test tools into a caller-specific extraction
# directory. The caller owns that directory and must call cleanup_envtest_tools
# after it has finished using the assets.
function fetch_tools {
  if [ -n "$SKIP_FETCH_TOOLS" ]; then
    if validate_envtest_assets "$envtest_bin_dir" 2>/dev/null; then
      return 0
    fi

    # Preserve the original skip-fetch location for callers that still
    # provision their own kubebuilder assets there.
    local legacy_envtest_bin_dir="$tmp_root/kubebuilder/bin"
    if validate_envtest_assets "$legacy_envtest_bin_dir" 2>/dev/null; then
      envtest_bin_dir=$legacy_envtest_bin_dir
      return 0
    fi

    validate_envtest_assets "$envtest_bin_dir"
    return 1
  fi

  header_text "fetching tools"
  local envtest_archive_name="envtest-v$k8s_version-$goos-$goarch.tar.gz"
  local envtest_download_url="https://github.com/kubernetes-sigs/controller-tools/releases/download/envtest-v$k8s_version/$envtest_archive_name"
  local envtest_archive_path="$tmp_root/$envtest_archive_name"
  local archive_to_extract=$envtest_archive_path
  local download_path=''
  local extraction_dir=''

  if ! mkdir -p "$tmp_root"; then
    echo "failed to create envtest cache directory $tmp_root" >&2
    return 1
  fi

  if [ -e "$envtest_archive_path" ] || [ -L "$envtest_archive_path" ]; then
    if [ ! -f "$envtest_archive_path" ] || [ -L "$envtest_archive_path" ]; then
      echo "envtest archive cache $envtest_archive_path is not a regular file" >&2
      return 1
    fi
  else
    if ! download_path=$(mktemp "$envtest_archive_path.XXXXXX"); then
      echo "failed to create envtest download staging file in $tmp_root" >&2
      return 1
    fi
    if ! curl -fsL "$envtest_download_url" -o "$download_path"; then
      rm -f "$download_path"
      echo "failed to download envtest tools from $envtest_download_url" >&2
      return 1
    fi
    archive_to_extract=$download_path
  fi

  if ! extraction_dir=$(mktemp -d "$tmp_root/envtest-tools.XXXXXX"); then
    if [ -n "$download_path" ]; then
      rm -f "$download_path"
    fi
    echo "failed to create envtest extraction directory in $tmp_root" >&2
    return 1
  fi
  if ! extract_envtest_archive "$archive_to_extract" "$extraction_dir"; then
    rm -rf "$extraction_dir"
    if [ -n "$download_path" ]; then
      rm -f "$download_path"
    fi
    return 1
  fi

  if [ -n "$download_path" ]; then
    if ! chmod 0644 "$download_path"; then
      rm -rf "$extraction_dir"
      rm -f "$download_path"
      echo "failed to set shared permissions on envtest archive candidate $download_path" >&2
      return 1
    fi
    if ln "$download_path" "$envtest_archive_path" 2>/dev/null; then
      if ! rm -f "$download_path"; then
        rm -rf "$extraction_dir"
        echo "failed to remove envtest download staging file $download_path" >&2
        return 1
      fi
      download_path=''
    elif [ -f "$envtest_archive_path" ] && [ ! -L "$envtest_archive_path" ]; then
      # Another fetch may have won publication. Validate its immutable cache
      # entry before accepting the race; never delete or replace it.
      local cache_validation_dir
      if ! cache_validation_dir=$(mktemp -d "$tmp_root/envtest-cache-validation.XXXXXX"); then
        rm -rf "$extraction_dir"
        rm -f "$download_path"
        echo "failed to create envtest cache validation directory in $tmp_root" >&2
        return 1
      fi
      if ! extract_envtest_archive "$envtest_archive_path" "$cache_validation_dir"; then
        rm -rf "$cache_validation_dir" "$extraction_dir"
        rm -f "$download_path"
        echo "failed to validate concurrently published envtest cache $envtest_archive_path" >&2
        return 1
      fi
      rm -rf "$cache_validation_dir"
      if ! rm -f "$download_path"; then
        rm -rf "$extraction_dir"
        echo "failed to remove envtest download staging file $download_path" >&2
        return 1
      fi
      download_path=''
    else
      rm -rf "$extraction_dir"
      rm -f "$download_path"
      echo "failed to publish envtest archive cache $envtest_archive_path" >&2
      return 1
    fi
  fi

  envtest_cleanup_dir=$extraction_dir
  envtest_bin_dir=$extraction_dir/controller-tools/envtest
}

function setup_envs {
  if ! validate_envtest_assets "$envtest_bin_dir"; then
    return 1
  fi

  header_text "setting up env vars"

  # Setup env vars
  export PATH="$envtest_bin_dir:$PATH"
  export TEST_ASSET_KUBECTL="$envtest_bin_dir/kubectl"
  export TEST_ASSET_KUBE_APISERVER="$envtest_bin_dir/kube-apiserver"
  export TEST_ASSET_ETCD="$envtest_bin_dir/etcd"
  export KUBEBUILDER_CONTROLPLANE_START_TIMEOUT=10m
  export NO_DOCKER=1

  # Ensure that some home var is set and that it's not the root
  export HOME=${HOME:=/tmp/kubebuilder-testing}
  if [ "$HOME" == "/" ]; then
    export HOME=/tmp/kubebuilder-testing
  fi
}
