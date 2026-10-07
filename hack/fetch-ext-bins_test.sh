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

# The single-quoted strings below intentionally generate scripts whose
# variables expand when those fixtures run, and every test subshell deliberately
# isolates its environment from the rest of the suite.
# shellcheck disable=SC2016,SC2030,SC2031

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
FETCH_SCRIPT="$REPO_ROOT/hack/fetch-ext-bins.sh"
CI_TEST_SCRIPT="$REPO_ROOT/hack/ci-test.sh"

function fail {
  echo "FAIL: $*" >&2
  return 1
}

function make_fixture_assets {
  local asset_dir=$1
  local entry_type=${2:-executable}
  local malformed_binary=${3:-}
  local binary
  local binary_entry_type

  mkdir -p "$asset_dir"
  for binary in kubectl kube-apiserver etcd; do
    binary_entry_type=executable
    if [ -z "$malformed_binary" ] || [ "$binary" = "$malformed_binary" ]; then
      binary_entry_type=$entry_type
    fi
    case "$binary_entry_type" in
      executable)
        printf '#!/usr/bin/env bash\nexit 0\n' >"$asset_dir/$binary"
        chmod +x "$asset_dir/$binary"
        ;;
      non-executable)
        printf '#!/usr/bin/env bash\nexit 0\n' >"$asset_dir/$binary"
        chmod -x "$asset_dir/$binary"
        ;;
      directory)
        mkdir "$asset_dir/$binary"
        ;;
      symlink)
        ln -s /bin/true "$asset_dir/$binary"
        ;;
      *)
        fail "unknown fixture entry type $binary_entry_type"
        ;;
    esac
  done
}

function make_fixture_archive {
  local fixture_root=$1
  local archive_path=$2
  local entry_type=${3:-executable}
  local malformed_binary=${4:-}

  make_fixture_assets "$fixture_root/controller-tools/envtest" "$entry_type" "$malformed_binary"
  tar -czf "$archive_path" -C "$fixture_root" controller-tools
}

function make_ancestor_symlink_archive {
  local fixture_root=$1
  local archive_path=$2
  local symlink_component=$3
  local external_dir="$fixture_root/external-$symlink_component"

  case "$symlink_component" in
    controller-tools)
      make_fixture_assets "$external_dir/envtest"
      ln -s "$external_dir" "$fixture_root/controller-tools"
      ;;
    envtest)
      make_fixture_assets "$external_dir"
      mkdir -p "$fixture_root/controller-tools"
      ln -s "$external_dir" "$fixture_root/controller-tools/envtest"
      ;;
    *) fail "unknown symlink component $symlink_component" ;;
  esac
  tar -czf "$archive_path" -C "$fixture_root" controller-tools
}

function assert_assets {
  local asset_dir=$1
  local binary

  for binary in kubectl kube-apiserver etcd; do
    [ -f "$asset_dir/$binary" ] || fail "$asset_dir/$binary is not a regular file"
    [ -x "$asset_dir/$binary" ] || fail "$asset_dir/$binary is not executable"
  done
}

function assert_no_extractions {
  local cache_root=$1
  local path

  for path in "$cache_root"/envtest-tools.* "$cache_root"/envtest-cache-validation.*; do
    if [ -e "$path" ] || [ -L "$path" ]; then
      fail "temporary envtest extraction path remains: $path"
    fi
  done
}

function assert_no_download_staging {
  local cache_root=$1
  local path

  for path in "$cache_root"/envtest-v*.tar.gz.*; do
    if [ -e "$path" ] || [ -L "$path" ]; then
      fail "temporary envtest download path remains: $path"
    fi
  done
}

function assert_shared_cache_permissions {
  local archive=$1

  case "$(LC_ALL=C ls -l "$archive")" in
    -rw-r--r--*) ;;
    *) fail "envtest cache $archive does not have mode 0644" ;;
  esac
}

function wait_for_path {
  local path=$1
  local attempt=0

  while [ "$attempt" -lt 200 ]; do
    [ -e "$path" ] && return 0
    attempt=$((attempt + 1))
    sleep 0.05
  done
  fail "timed out waiting for $path"
}

function write_curl_stub {
  local bin_dir=$1

  mkdir -p "$bin_dir"
  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'set -o errexit' \
    '[ "$#" -eq 4 ]' \
    '[ "$1" = "-fsL" ]' \
    '[ "$3" = "-o" ]' \
    'cp "$FIXTURE_ARCHIVE" "$4"' \
    'if [ -n "${DOWNLOAD_SYNC_DIR:-}" ]; then' \
    '  touch "$DOWNLOAD_SYNC_DIR/download.$$"' \
    '  attempt=0' \
    '  while [ "$attempt" -lt 200 ]; do' \
    '    set -- "$DOWNLOAD_SYNC_DIR"/download.*' \
    '    [ "$#" -ge 2 ] && exit 0' \
    '    attempt=$((attempt + 1))' \
    '    sleep 0.05' \
    '  done' \
    '  exit 1' \
    'fi' >"$bin_dir/curl"
  chmod +x "$bin_dir/curl"
}

function test_platform_download_and_cache_reuse {
  local platform=$1
  local expected_url=$2

  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local fixture_archive="$test_root/fixture.tar.gz"
    make_fixture_archive "$test_root/fixture" "$fixture_archive"

    local ENVTEST_TMP_ROOT="$test_root/cache"
    export ENVTEST_TMP_ROOT
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"
    goos=$platform

    # shellcheck disable=SC2317 # fetch_tools invokes this curl stub.
    function curl {
      [ "$#" -eq 4 ] || fail "curl received an unexpected argument count: $#"
      [ "$1" = "-fsL" ] || fail "curl did not receive -fsL"
      [ "$2" = "$expected_url" ] || fail "unexpected download URL: $2"
      [ "$3" = "-o" ] || fail "curl did not receive -o"
      cp "$fixture_archive" "$4"
    }

    fetch_tools
    local first_bin_dir=$envtest_bin_dir
    setup_envs
    assert_assets "$first_bin_dir"
    [ "$TEST_ASSET_KUBECTL" = "$first_bin_dir/kubectl" ] || fail "TEST_ASSET_KUBECTL has the wrong path"
    local archive="$ENVTEST_TMP_ROOT/envtest-v1.23.5-$platform-amd64.tar.gz"
    [ -f "$archive" ] || fail "validated archive was not cached"
    assert_shared_cache_permissions "$archive"
    cleanup_envtest_tools
    [ ! -e "$(dirname "$(dirname "$first_bin_dir")")" ] || fail "first extraction was not cleaned up"

    # shellcheck disable=SC2317 # A cache hit must not invoke this curl stub.
    function curl {
      fail "cached archive was downloaded again"
    }
    fetch_tools
    [ "$envtest_bin_dir" != "$first_bin_dir" ] || fail "fetches reused a mutable extraction path"
    assert_assets "$envtest_bin_dir"
    cleanup_envtest_tools
    assert_no_extractions "$ENVTEST_TMP_ROOT"
  )
}

function test_failed_download {
  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local ENVTEST_TMP_ROOT="$test_root/cache"
    export ENVTEST_TMP_ROOT
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"

    # shellcheck disable=SC2317 # fetch_tools invokes this curl stub.
    function curl {
      printf 'partial download\n' >"$4"
      return 22
    }

    if fetch_tools >/dev/null 2>&1; then
      fail "fetch_tools succeeded after a failed download"
    fi
    [ ! -e "$ENVTEST_TMP_ROOT/envtest-v$k8s_version-$goos-$goarch.tar.gz" ] || fail "failed download was cached"
    assert_no_download_staging "$ENVTEST_TMP_ROOT"
    assert_no_extractions "$ENVTEST_TMP_ROOT"
  )
}

function test_corrupt_cache_is_not_deleted {
  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local ENVTEST_TMP_ROOT="$test_root/cache"
    export ENVTEST_TMP_ROOT
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"

    mkdir -p "$ENVTEST_TMP_ROOT/kubebuilder/bin"
    touch "$ENVTEST_TMP_ROOT/kubebuilder/bin/kubectl"
    chmod +x "$ENVTEST_TMP_ROOT/kubebuilder/bin/kubectl"
    local archive="$ENVTEST_TMP_ROOT/envtest-v$k8s_version-$goos-$goarch.tar.gz"
    printf 'not a tar archive\n' >"$archive"

    if fetch_tools >/dev/null 2>&1; then
      fail "fetch_tools succeeded with a corrupt archive"
    fi
    grep -q 'not a tar archive' "$archive" || fail "corrupt cache was destructively removed or replaced"
    [ -x "$ENVTEST_TMP_ROOT/kubebuilder/bin/kubectl" ] || fail "legacy caller assets were changed"
    assert_no_extractions "$ENVTEST_TMP_ROOT"
  )
}

function test_malformed_cached_archive_entry {
  local entry_type=$1
  local malformed_binary=$2

  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local ENVTEST_TMP_ROOT="$test_root/cache"
    export ENVTEST_TMP_ROOT
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"

    mkdir -p "$ENVTEST_TMP_ROOT"
    local archive="$ENVTEST_TMP_ROOT/envtest-v$k8s_version-$goos-$goarch.tar.gz"
    make_fixture_archive "$test_root/fixture" "$archive" "$entry_type" "$malformed_binary"

    if fetch_tools >/dev/null 2>&1; then
      fail "fetch_tools accepted a $entry_type $malformed_binary archive entry"
    fi
    [ -f "$archive" ] || fail "$entry_type $malformed_binary cache was destructively removed"
    assert_no_extractions "$ENVTEST_TMP_ROOT"
  )
}

function test_malformed_downloaded_archive_entry {
  local entry_type=$1
  local malformed_binary=$2

  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local fixture_archive="$test_root/fixture.tar.gz"
    make_fixture_archive "$test_root/fixture" "$fixture_archive" "$entry_type" "$malformed_binary"
    local ENVTEST_TMP_ROOT="$test_root/cache"
    export ENVTEST_TMP_ROOT
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"
    local archive="$ENVTEST_TMP_ROOT/envtest-v$k8s_version-$goos-$goarch.tar.gz"

    # shellcheck disable=SC2317 # fetch_tools invokes this curl stub.
    function curl { cp "$fixture_archive" "$4"; }

    if fetch_tools >/dev/null 2>&1; then
      fail "fetch_tools published a downloaded $entry_type $malformed_binary archive entry"
    fi
    if [ -e "$archive" ] || [ -L "$archive" ]; then
      fail "downloaded $entry_type $malformed_binary archive was published"
    fi
    assert_no_download_staging "$ENVTEST_TMP_ROOT"
    assert_no_extractions "$ENVTEST_TMP_ROOT"
  )
}

function test_ancestor_symlink_archive {
  local archive_source=$1
  local symlink_component=$2

  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local fixture_archive="$test_root/fixture.tar.gz"
    make_ancestor_symlink_archive "$test_root/fixture" "$fixture_archive" "$symlink_component"
    local ENVTEST_TMP_ROOT="$test_root/cache"
    export ENVTEST_TMP_ROOT
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"
    local archive="$ENVTEST_TMP_ROOT/envtest-v$k8s_version-$goos-$goarch.tar.gz"

    mkdir -p "$ENVTEST_TMP_ROOT"
    if [ "$archive_source" = cached ]; then
      cp "$fixture_archive" "$archive"
      # shellcheck disable=SC2317 # Cached archives must not invoke curl.
      function curl { fail "cached ancestor-symlink archive triggered a download"; }
    else
      # shellcheck disable=SC2317 # fetch_tools invokes this curl stub.
      function curl { cp "$fixture_archive" "$4"; }
    fi

    if fetch_tools >/dev/null 2>&1; then
      fail "fetch_tools accepted a $archive_source $symlink_component symlink"
    fi
    if [ "$archive_source" = cached ]; then
      [ -f "$archive" ] || fail "$symlink_component symlink cache was destructively removed"
    elif [ -e "$archive" ] || [ -L "$archive" ]; then
      fail "downloaded $symlink_component symlink archive was published"
    fi
    assert_no_download_staging "$ENVTEST_TMP_ROOT"
    assert_no_extractions "$ENVTEST_TMP_ROOT"
  )
}

function test_cache_permission_failure {
  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local fixture_archive="$test_root/fixture.tar.gz"
    make_fixture_archive "$test_root/fixture" "$fixture_archive"
    local ENVTEST_TMP_ROOT="$test_root/cache"
    export ENVTEST_TMP_ROOT
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"
    local archive="$ENVTEST_TMP_ROOT/envtest-v$k8s_version-$goos-$goarch.tar.gz"

    # shellcheck disable=SC2317 # fetch_tools invokes these stubs.
    function curl { cp "$fixture_archive" "$4"; }
    function chmod { return 1; }

    if fetch_tools >/dev/null 2>&1; then
      fail "fetch_tools succeeded after cache permission setup failed"
    fi
    if [ -e "$archive" ] || [ -L "$archive" ]; then
      fail "archive was published after cache permission setup failed"
    fi
    assert_no_download_staging "$ENVTEST_TMP_ROOT"
    assert_no_extractions "$ENVTEST_TMP_ROOT"
  )
}

function test_publication_failure_in_and_list {
  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local fixture_archive="$test_root/fixture.tar.gz"
    make_fixture_archive "$test_root/fixture" "$fixture_archive"
    local ENVTEST_TMP_ROOT="$test_root/cache"
    export ENVTEST_TMP_ROOT
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"
    local publication_attempted=0

    # shellcheck disable=SC2317 # fetch_tools invokes these stubs.
    function curl { cp "$fixture_archive" "$4"; }
    function ln {
      publication_attempted=1
      return 1
    }

    if fetch_tools && setup_envs; then
      fail "fetch_tools succeeded in an AND list after publication failed"
    fi
    [ "$publication_attempted" -eq 1 ] || fail "archive publication failure was not exercised"
    assert_no_extractions "$ENVTEST_TMP_ROOT"
    unset -f ln

    fetch_tools && setup_envs
    assert_assets "$envtest_bin_dir"
    cleanup_envtest_tools
  )
}

function test_download_staging_cleanup_failure_in_and_list {
  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local fixture_archive="$test_root/fixture.tar.gz"
    make_fixture_archive "$test_root/fixture" "$fixture_archive"
    local ENVTEST_TMP_ROOT="$test_root/cache"
    export ENVTEST_TMP_ROOT
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"
    local archive="$ENVTEST_TMP_ROOT/envtest-v$k8s_version-$goos-$goarch.tar.gz"
    local output
    local status=0

    # shellcheck disable=SC2317 # fetch_tools invokes these stubs.
    function curl { cp "$fixture_archive" "$4"; }
    function rm {
      case "$*" in
        *envtest-v*.tar.gz.*) return 19 ;;
      esac
      command rm "$@"
    }

    output=$({ fetch_tools && setup_envs; } 2>&1) || status=$?
    [ "$status" -ne 0 ] || fail "fetch_tools succeeded in an AND list after download staging cleanup failed"
    case "$output" in
      *"failed to remove envtest download staging file "*) ;;
      *) fail "fetch_tools did not report the download staging cleanup failure" ;;
    esac
    [ -f "$archive" ] || fail "download staging cleanup failure removed the published cache"
    set -- "$ENVTEST_TMP_ROOT"/envtest-v*.tar.gz.*
    if [ "$#" -ne 1 ] || [ ! -f "$1" ]; then
      fail "download staging cleanup failure was not exercised"
    fi
    assert_no_extractions "$ENVTEST_TMP_ROOT"
  )
}

function test_publication_loser_rejects_malformed_winner {
  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local fixture_archive="$test_root/fixture.tar.gz"
    local malformed_archive="$test_root/malformed.tar.gz"
    make_fixture_archive "$test_root/fixture" "$fixture_archive"
    make_fixture_archive "$test_root/malformed" "$malformed_archive" non-executable etcd
    local ENVTEST_TMP_ROOT="$test_root/cache"
    export ENVTEST_TMP_ROOT
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"
    local archive="$ENVTEST_TMP_ROOT/envtest-v$k8s_version-$goos-$goarch.tar.gz"

    # shellcheck disable=SC2317 # fetch_tools invokes these stubs.
    function curl { cp "$fixture_archive" "$4"; }
    function ln {
      cp "$malformed_archive" "$2"
      return 1
    }

    if fetch_tools >/dev/null 2>&1; then
      fail "publication loser accepted a malformed race winner"
    fi
    cmp -s "$malformed_archive" "$archive" || fail "publication loser changed the malformed race winner"
    assert_no_download_staging "$ENVTEST_TMP_ROOT"
    assert_no_extractions "$ENVTEST_TMP_ROOT"
  )
}

function test_invalid_cache_target {
  local target_type=$1

  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local ENVTEST_TMP_ROOT="$test_root/cache"
    export ENVTEST_TMP_ROOT
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"
    mkdir -p "$ENVTEST_TMP_ROOT"
    local archive="$ENVTEST_TMP_ROOT/envtest-v$k8s_version-$goos-$goarch.tar.gz"
    case "$target_type" in
      directory) mkdir "$archive" ;;
      symlink)
        printf 'target\n' >"$test_root/target"
        ln -s "$test_root/target" "$archive"
        ;;
    esac

    # shellcheck disable=SC2317 # Invalid cache targets must fail before curl.
    function curl { fail "invalid cache target triggered a download"; }
    if fetch_tools >/dev/null 2>&1; then
      fail "fetch_tools accepted a $target_type cache target"
    fi
    [ -e "$archive" ] || [ -L "$archive" ] || fail "$target_type cache target was deleted"
  )
}

function test_concurrent_callers_use_distinct_assets {
  (
    local test_root
    test_root=$(mktemp -d)
    trap 'touch "$test_root/one.release" "$test_root/two.release" 2>/dev/null || true; rm -rf "$test_root"' EXIT

    local fixture_archive="$test_root/fixture.tar.gz"
    make_fixture_archive "$test_root/fixture" "$fixture_archive"
    write_curl_stub "$test_root/bin"
    mkdir -p "$test_root/sync"
    export FIXTURE_ARCHIVE="$fixture_archive"
    export DOWNLOAD_SYNC_DIR="$test_root/sync"
    export ENVTEST_TMP_ROOT="$test_root/cache"
    export FETCH_SCRIPT
    export PATH="$test_root/bin:$PATH"
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"

    local worker='source "$FETCH_SCRIPT"; fetch_tools; setup_envs; assert_dir=$envtest_bin_dir; printf "%s\n" "$assert_dir" >"$OUTPUT_FILE"; touch "$READY_FILE"; while [ ! -e "$RELEASE_FILE" ]; do sleep 0.05; done; validate_envtest_assets "$assert_dir"; cleanup_envtest_tools'
    OUTPUT_FILE="$test_root/one.path" READY_FILE="$test_root/one.ready" RELEASE_FILE="$test_root/one.release" bash -c "$worker" &
    local first_pid=$!
    OUTPUT_FILE="$test_root/two.path" READY_FILE="$test_root/two.ready" RELEASE_FILE="$test_root/two.release" bash -c "$worker" &
    local second_pid=$!

    wait_for_path "$test_root/one.ready"
    wait_for_path "$test_root/two.ready"
    local first_dir second_dir
    first_dir=$(cat "$test_root/one.path")
    second_dir=$(cat "$test_root/two.path")
    [ "$first_dir" != "$second_dir" ] || fail "concurrent callers shared an extraction directory"
    assert_assets "$first_dir"
    assert_assets "$second_dir"
    [ -f "$ENVTEST_TMP_ROOT/envtest-v$k8s_version-$goos-$goarch.tar.gz" ] || fail "concurrent publication did not produce a regular cache file"

    touch "$test_root/one.release"
    local first_status=0 second_status=0
    wait "$first_pid" || first_status=$?
    [ "$first_status" -eq 0 ] || fail "first concurrent caller failed with $first_status"
    [ ! -e "$(dirname "$(dirname "$first_dir")")" ] || fail "first concurrent caller did not clean up its assets"
    assert_assets "$second_dir"
    "$second_dir/kubectl" || fail "second concurrent caller's active asset was not executable after first cleanup"

    touch "$test_root/two.release"
    wait "$second_pid" || second_status=$?
    [ "$second_status" -eq 0 ] || fail "second concurrent caller failed with $second_status"
    assert_no_extractions "$ENVTEST_TMP_ROOT"
  )
}

function test_interrupted_caller_does_not_block_later_fetch {
  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local fixture_archive="$test_root/fixture.tar.gz"
    make_fixture_archive "$test_root/fixture" "$fixture_archive"
    write_curl_stub "$test_root/bin"
    export FIXTURE_ARCHIVE="$fixture_archive"
    export DOWNLOAD_SYNC_DIR=''
    export ENVTEST_TMP_ROOT="$test_root/cache"
    export FETCH_SCRIPT
    export PATH="$test_root/bin:$PATH"

    READY_FILE="$test_root/interrupted.ready" bash -c 'source "$FETCH_SCRIPT"; fetch_tools; touch "$READY_FILE"; while :; do sleep 1; done' &
    local interrupted_pid=$!
    wait_for_path "$test_root/interrupted.ready"
    kill -KILL "$interrupted_pid"
    wait "$interrupted_pid" 2>/dev/null || true

    # A stale directory from the removed lock protocol must also be irrelevant.
    mkdir -p "$ENVTEST_TMP_ROOT/controller-tools/.envtest-install.lock"
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"
    fetch_tools && setup_envs
    assert_assets "$envtest_bin_dir"
    cleanup_envtest_tools
  )
}

function test_skip_with_valid_assets_unchanged {
  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local ENVTEST_TMP_ROOT="$test_root/cache"
    local SKIP_FETCH_TOOLS=1
    export ENVTEST_TMP_ROOT SKIP_FETCH_TOOLS
    local caller_dir="$ENVTEST_TMP_ROOT/controller-tools/envtest"
    make_fixture_assets "$caller_dir"
    printf 'caller-owned\n' >"$caller_dir/marker"
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"

    # shellcheck disable=SC2317 # Skip mode must not invoke curl.
    function curl { fail "skip mode attempted a download"; }

    fetch_tools && setup_envs
    [ "$envtest_bin_dir" = "$caller_dir" ] || fail "skip mode changed the caller-managed asset path"
    cleanup_envtest_tools
    grep -q 'caller-owned' "$caller_dir/marker" || fail "skip mode changed caller-managed assets"
    assert_assets "$caller_dir"
  )
}

function test_skip_with_symlink_assets_unchanged {
  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local ENVTEST_TMP_ROOT="$test_root/cache"
    local SKIP_FETCH_TOOLS=1
    export ENVTEST_TMP_ROOT SKIP_FETCH_TOOLS
    local caller_dir="$ENVTEST_TMP_ROOT/controller-tools/envtest"
    make_fixture_assets "$caller_dir" symlink
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"

    fetch_tools && setup_envs
    [ "$envtest_bin_dir" = "$caller_dir" ] || fail "skip mode changed the caller-managed symlink asset path"
    [ -L "$caller_dir/kubectl" ] || fail "skip mode replaced a caller-managed symlink asset"
    cleanup_envtest_tools
    assert_assets "$caller_dir"
  )
}

function test_skip_with_legacy_assets {
  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local ENVTEST_TMP_ROOT="$test_root/cache"
    local SKIP_FETCH_TOOLS=1
    export ENVTEST_TMP_ROOT SKIP_FETCH_TOOLS
    local legacy_dir="$ENVTEST_TMP_ROOT/kubebuilder/bin"
    make_fixture_assets "$legacy_dir"
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"

    fetch_tools && setup_envs
    [ "$envtest_bin_dir" = "$legacy_dir" ] || fail "skip mode did not preserve the legacy kubebuilder asset path"
    [ "$TEST_ASSET_ETCD" = "$legacy_dir/etcd" ] || fail "legacy skip mode exported the wrong etcd path"
    cleanup_envtest_tools
    assert_assets "$legacy_dir"
  )
}

function test_skip_with_invalid_assets {
  local entry_type=$1

  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local ENVTEST_TMP_ROOT="$test_root/cache"
    local SKIP_FETCH_TOOLS=1
    export ENVTEST_TMP_ROOT SKIP_FETCH_TOOLS
    if [ "$entry_type" != "missing" ]; then
      make_fixture_assets "$ENVTEST_TMP_ROOT/controller-tools/envtest" "$entry_type"
    fi
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"

    if fetch_tools && setup_envs; then
      fail "skip mode accepted $entry_type assets"
    fi
  )
}

function test_sourcing_preserves_caller_trap {
  (
    trap 'true' EXIT
    local before after
    before=$(trap -p EXIT)
    # shellcheck source=hack/fetch-ext-bins.sh
    source "$FETCH_SCRIPT"
    after=$(trap -p EXIT)
    [ "$before" = "$after" ] || fail "fetch helper overwrote its caller's EXIT trap"
  )
}

function test_ci_cleanup_preserves_test_status {
  local requested_status=$1
  local cleanup_status=${2:-0}
  local expected_status=${3:-$requested_status}

  (
    local test_root
    test_root=$(mktemp -d)
    trap 'rm -rf "$test_root"' EXIT

    local fixture_archive="$test_root/fixture.tar.gz"
    make_fixture_archive "$test_root/fixture" "$fixture_archive"
    write_curl_stub "$test_root/bin"
    local real_rm
    real_rm=$(command -v rm)
    if [ "$cleanup_status" -ne 0 ]; then
      printf '%s\n' \
        '#!/usr/bin/env bash' \
        'case "$*" in' \
        '  *envtest-tools.*) exit "$RM_STATUS" ;;' \
        'esac' \
        'exec "$REAL_RM" "$@"' >"$test_root/bin/rm"
      chmod +x "$test_root/bin/rm"
    fi
    printf '%s\n' \
      '#!/usr/bin/env bash' \
      '[ "$1" = "unit" ]' \
      'asset_root=$(dirname "$(dirname "$(dirname "$TEST_ASSET_KUBECTL")")")' \
      'printf "%s\n" "$asset_root" >"$MAKE_RECORD"' \
      'exit "$MAKE_STATUS"' >"$test_root/bin/make"
    chmod +x "$test_root/bin/make"

    local status=0
    PATH="$test_root/bin:$PATH" \
      FIXTURE_ARCHIVE="$fixture_archive" \
      DOWNLOAD_SYNC_DIR='' \
      ENVTEST_TMP_ROOT="$test_root/cache" \
      MAKE_RECORD="$test_root/make.record" \
      MAKE_STATUS="$requested_status" \
      REAL_RM="$real_rm" \
      RM_STATUS="$cleanup_status" \
      NO_COLOR=1 \
      "$CI_TEST_SCRIPT" >/dev/null 2>&1 || status=$?

    [ "$status" -eq "$expected_status" ] || fail "ci-test returned $status instead of $expected_status for unit status $requested_status and cleanup status $cleanup_status"
    local asset_root
    asset_root=$(cat "$test_root/make.record")
    if [ "$cleanup_status" -eq 0 ]; then
      [ ! -e "$asset_root" ] || fail "ci-test left its extraction after unit status $requested_status"
    else
      [ -d "$asset_root" ] || fail "cleanup failure fixture did not preserve the extraction"
    fi
  )
}

test_platform_download_and_cache_reuse linux "https://github.com/kubernetes-sigs/controller-tools/releases/download/envtest-v1.23.5/envtest-v1.23.5-linux-amd64.tar.gz"
test_platform_download_and_cache_reuse darwin "https://github.com/kubernetes-sigs/controller-tools/releases/download/envtest-v1.23.5/envtest-v1.23.5-darwin-amd64.tar.gz"
test_failed_download
test_corrupt_cache_is_not_deleted
for entry_type in directory non-executable symlink; do
  for binary in kubectl kube-apiserver etcd; do
    test_malformed_cached_archive_entry "$entry_type" "$binary"
    test_malformed_downloaded_archive_entry "$entry_type" "$binary"
  done
done
for symlink_component in controller-tools envtest; do
  test_ancestor_symlink_archive cached "$symlink_component"
  test_ancestor_symlink_archive downloaded "$symlink_component"
done
test_cache_permission_failure
test_publication_failure_in_and_list
test_download_staging_cleanup_failure_in_and_list
test_publication_loser_rejects_malformed_winner
test_invalid_cache_target directory
test_invalid_cache_target symlink
test_concurrent_callers_use_distinct_assets
test_interrupted_caller_does_not_block_later_fetch
test_skip_with_valid_assets_unchanged
test_skip_with_symlink_assets_unchanged
test_skip_with_legacy_assets
test_skip_with_invalid_assets missing
test_skip_with_invalid_assets non-executable
test_sourcing_preserves_caller_trap
test_ci_cleanup_preserves_test_status 0
test_ci_cleanup_preserves_test_status 37
test_ci_cleanup_preserves_test_status 0 19 1
test_ci_cleanup_preserves_test_status 37 19 37
echo "fetch-ext-bins tests passed"
