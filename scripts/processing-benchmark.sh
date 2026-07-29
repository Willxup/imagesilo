#!/usr/bin/env bash
set -euo pipefail

image="${IMAGE:-imagesilo:phase3-probe}"
platform="${PLATFORM:-linux/arm64}"
port="${PORT:-18086}"
suffix="${BENCH_SUFFIX:-local}"
container="imagesilo-processing-bench-${suffix}"
volume="${container}-data"
password="ImageSilo-${suffix}-Benchmark-Password!"
concurrencies="${CONCURRENCIES:-1}"
requests="${REQUESTS:-16}"
diagnostics="${BENCH_DIAGNOSTICS:-false}"
fixture_width="${FIXTURE_WIDTH:-5000}"
fixture_height="${FIXTURE_HEIGHT:-3200}"
conversion_enabled="${CONVERSION_ENABLED:-true}"

cleanup() {
  docker rm --force "$container" >/dev/null 2>&1 || true
  docker volume rm "$volume" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

wait_ready() {
  local base_url="$1"
  for _ in $(seq 1 60); do
    if curl --fail --silent "${base_url}/readyz" >/dev/null; then
      return 0
    fi
    sleep 1
  done
  docker logs "$container" || true
  return 1
}

cleanup
docker volume create "$volume" >/dev/null
printf '%s\n' "$password" | docker run --rm --interactive \
  --platform "$platform" \
  --volume "${volume}:/data" \
  "$image" admin create --email admin@example.com --password-stdin

for concurrency in $concurrencies; do
  docker run --detach --rm \
    --platform "$platform" \
    --name "$container" \
    --publish "127.0.0.1:${port}:8080" \
    --env IMAGESILO_COOKIE_SECURE=false \
    --env "IMAGESILO_PROCESSING_CONCURRENCY=${concurrency}" \
    --volume "${volume}:/data" \
    "$image" >/dev/null
  base_url="http://127.0.0.1:${port}"
  wait_ready "$base_url"
  IMAGESILO_BENCH_PASSWORD="$password" go run ./tests/performance/upload_benchmark.go \
    -base-url "$base_url" -concurrency "$concurrency" -requests "$requests" \
    -width "$fixture_width" -height "$fixture_height" -conversion-enabled="$conversion_enabled"
  memory_current="$(docker exec "$container" sh -c 'cat /sys/fs/cgroup/memory.current')"
  memory_peak="$(docker exec "$container" sh -c 'cat /sys/fs/cgroup/memory.peak')"
  memory_anon="$(docker exec "$container" sh -c "awk '\$1 == \"anon\" { print \$2 }' /sys/fs/cgroup/memory.stat")"
  memory_file="$(docker exec "$container" sh -c "awk '\$1 == \"file\" { print \$2 }' /sys/fs/cgroup/memory.stat")"
  printf '{"concurrency":%s,"memoryCurrentBytes":%s,"memoryPeakBytes":%s,"memoryAnonBytes":%s,"memoryFileBytes":%s}\n' \
    "$concurrency" "$memory_current" "$memory_peak" "$memory_anon" "$memory_file"
  if test "$diagnostics" = "true"; then
    docker logs "$container" 2>&1 | grep 'image upload completed' | tail -n 1 >&2 || true
    docker exec "$container" sh -c 'cat /sys/fs/cgroup/memory.stat' >&2
  fi
  docker stop --timeout 15 "$container" >/dev/null
done
