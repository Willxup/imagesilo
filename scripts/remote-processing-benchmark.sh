#!/usr/bin/env bash
set -euo pipefail

image="${IMAGE:?IMAGE is required}"
bench_client="${BENCH_CLIENT:?BENCH_CLIENT is required}"
work_dir="${WORK_DIR:?WORK_DIR is required}"
port="${PORT:-18086}"
suffix="${BENCH_SUFFIX:-remote}"
concurrencies="${CONCURRENCIES:-1}"
requests="${REQUESTS:-4}"
cpu_limit="${CPU_LIMIT:-0.5}"
memory_limit="${MEMORY_LIMIT:-1g}"
pids_limit="${PIDS_LIMIT:-256}"
fixture_width="${FIXTURE_WIDTH:-5000}"
fixture_height="${FIXTURE_HEIGHT:-3200}"
conversion_enabled="${CONVERSION_ENABLED:-true}"
max_memory_peak_bytes="${MAX_MEMORY_PEAK_BYTES:-536870912}"
max_p95_milliseconds="${MAX_P95_MILLISECONDS:-30000}"
if [[ ! "$suffix" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$ ]]; then
  printf 'BENCH_SUFFIX must contain only 1-64 safe name characters\n' >&2
  exit 1
fi

work_dir="$(realpath -m -- "$work_dir")"
case "$work_dir" in
  /var/tmp/imagesilo-?*) ;;
  *) printf 'WORK_DIR must be a dedicated /var/tmp/imagesilo-* directory\n' >&2; exit 1 ;;
esac
mkdir -p "$work_dir"
work_dir="$(realpath -- "$work_dir")"
case "$work_dir" in
  /var/tmp/imagesilo-?*) ;;
  *) printf 'canonical WORK_DIR escaped /var/tmp/imagesilo-*\n' >&2; exit 1 ;;
esac
if find "$work_dir" -mindepth 1 -print -quit | grep -q .; then
  printf 'WORK_DIR must be an empty dedicated directory: %s\n' "$work_dir" >&2
  exit 1
fi
container="imagesilo-processing-bench-${suffix}"
data_dir="${work_dir}/data"
result_file="${work_dir}/results/benchmark.jsonl"
password="ImageSilo-${suffix}-Benchmark-Password!"

cleanup() {
  docker rm --force "$container" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

wait_ready() {
  local base_url="$1"
  for _ in $(seq 1 120); do
    if curl --fail --silent "${base_url}/readyz" >/dev/null; then
      return 0
    fi
    sleep 1
  done
  docker logs "$container" || true
  return 1
}

mkdir -p "$data_dir/db" "$data_dir/images" "$data_dir/migrations" "$data_dir/cache/thumbnails" "$data_dir/tmp" "$(dirname "$result_file")"
chown 10001:10001 "$data_dir" "$data_dir/db" "$data_dir/images" "$data_dir/migrations" "$data_dir/cache" "$data_dir/cache/thumbnails" "$data_dir/tmp"
: >"$result_file"
cleanup
printf '%s\n' "$password" | docker run --rm --interactive \
  --cpus "$cpu_limit" \
  --memory "$memory_limit" \
  --pids-limit "$pids_limit" \
  --volume "${data_dir}:/data" \
  "$image" admin create --email admin@example.com --password-stdin

for concurrency in $concurrencies; do
  docker run --detach --rm \
    --name "$container" \
    --cpus "$cpu_limit" \
    --memory "$memory_limit" \
    --pids-limit "$pids_limit" \
    --publish "127.0.0.1:${port}:8080" \
    --env IMAGESILO_COOKIE_SECURE=false \
    --env "IMAGESILO_PROCESSING_CONCURRENCY=${concurrency}" \
    --volume "${data_dir}:/data" \
    "$image" >/dev/null
  base_url="http://127.0.0.1:${port}"
  wait_ready "$base_url"
  benchmark_output="${work_dir}/results/benchmark-${concurrency}.json"
  IMAGESILO_BENCH_PASSWORD="$password" "$bench_client" \
    -base-url "$base_url" -concurrency "$concurrency" -requests "$requests" \
  -width "$fixture_width" -height "$fixture_height" -conversion-enabled="$conversion_enabled" | tee "$benchmark_output" | tee -a "$result_file"
  python3 - "$benchmark_output" "$requests" "$max_p95_milliseconds" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as source:
    result = json.load(source)
requests = int(sys.argv[2])
maximum_p95 = float(sys.argv[3])
if result["requests"] != requests or result["successes"] != requests or result["busyResponses"] != 0:
    raise SystemExit("processing benchmark reported failed or busy uploads")
if result["p95Milliseconds"] > maximum_p95:
    raise SystemExit(f"processing benchmark p95 {result['p95Milliseconds']} exceeded {maximum_p95} ms")
PY
  memory_current="$(docker exec "$container" sh -c 'cat /sys/fs/cgroup/memory.current')"
  memory_peak="$(docker exec "$container" sh -c 'cat /sys/fs/cgroup/memory.peak')"
  memory_anon="$(docker exec "$container" sh -c "awk '\$1 == \"anon\" { print \$2 }' /sys/fs/cgroup/memory.stat")"
  memory_file="$(docker exec "$container" sh -c "awk '\$1 == \"file\" { print \$2 }' /sys/fs/cgroup/memory.stat")"
  if test "$memory_peak" -gt "$max_memory_peak_bytes"; then
    printf 'processing benchmark peak memory %s exceeded limit %s bytes\n' "$memory_peak" "$max_memory_peak_bytes" >&2
    exit 1
  fi
  printf '{"concurrency":%s,"memoryCurrentBytes":%s,"memoryPeakBytes":%s,"memoryAnonBytes":%s,"memoryFileBytes":%s}\n' \
    "$concurrency" "$memory_current" "$memory_peak" "$memory_anon" "$memory_file" | tee -a "$result_file"
  docker stop --timeout 15 "$container" >/dev/null
done

printf 'ImageSilo remote processing benchmark passed: result=%s\n' "$result_file"
