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
fixture_width="${FIXTURE_WIDTH:-5000}"
fixture_height="${FIXTURE_HEIGHT:-3200}"
conversion_enabled="${CONVERSION_ENABLED:-true}"
container="imagesilo-processing-bench-${suffix}"
data_dir="${work_dir}/data"
result_file="${work_dir}/results/benchmark.jsonl"
password="ImageSilo-${suffix}-Benchmark-Password!"

case "$work_dir" in
  /var/tmp/*) ;;
  *) printf 'WORK_DIR must be below /var/tmp\n' >&2; exit 1 ;;
esac

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

mkdir -p "$data_dir/db" "$data_dir/images" "$data_dir/cache/thumbnails" "$data_dir/tmp" "$(dirname "$result_file")"
chown -R 10001:10001 "$data_dir"
: >"$result_file"
cleanup
printf '%s\n' "$password" | docker run --rm --interactive \
  --cpus "$cpu_limit" \
  --memory "$memory_limit" \
  --pids-limit 256 \
  --volume "${data_dir}:/data" \
  "$image" admin create --email admin@example.com --password-stdin

for concurrency in $concurrencies; do
  docker run --detach --rm \
    --name "$container" \
    --cpus "$cpu_limit" \
    --memory "$memory_limit" \
    --pids-limit 256 \
    --publish "127.0.0.1:${port}:8080" \
    --env IMAGESILO_COOKIE_SECURE=false \
    --env "IMAGESILO_PROCESSING_CONCURRENCY=${concurrency}" \
    --volume "${data_dir}:/data" \
    "$image" >/dev/null
  base_url="http://127.0.0.1:${port}"
  wait_ready "$base_url"
  IMAGESILO_BENCH_PASSWORD="$password" "$bench_client" \
    -base-url "$base_url" -concurrency "$concurrency" -requests "$requests" \
    -width "$fixture_width" -height "$fixture_height" -conversion-enabled="$conversion_enabled" | tee -a "$result_file"
  memory_current="$(docker exec "$container" sh -c 'cat /sys/fs/cgroup/memory.current')"
  memory_peak="$(docker exec "$container" sh -c 'cat /sys/fs/cgroup/memory.peak')"
  memory_anon="$(docker exec "$container" sh -c "awk '\$1 == \"anon\" { print \$2 }' /sys/fs/cgroup/memory.stat")"
  memory_file="$(docker exec "$container" sh -c "awk '\$1 == \"file\" { print \$2 }' /sys/fs/cgroup/memory.stat")"
  printf '{"concurrency":%s,"memoryCurrentBytes":%s,"memoryPeakBytes":%s,"memoryAnonBytes":%s,"memoryFileBytes":%s}\n' \
    "$concurrency" "$memory_current" "$memory_peak" "$memory_anon" "$memory_file" | tee -a "$result_file"
  docker stop --timeout 15 "$container" >/dev/null
done

printf 'ImageSilo remote processing benchmark passed: result=%s\n' "$result_file"
