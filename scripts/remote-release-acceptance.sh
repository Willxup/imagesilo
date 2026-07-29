#!/usr/bin/env bash
set -euo pipefail

image="${IMAGE:?IMAGE is required}"
work_dir="${WORK_DIR:?WORK_DIR is required}"
port="${PORT:-18088}"
duration_seconds="${DURATION_SECONDS:-600}"
request_interval="${REQUEST_INTERVAL:-5}"
cpu_limit="${CPU_LIMIT:-0.5}"
memory_limit="${MEMORY_LIMIT:-256m}"
pids_limit="${PIDS_LIMIT:-128}"
suffix="${ACCEPTANCE_SUFFIX:-$(date -u +%Y%m%d%H%M%S)}"
container="imagesilo-acceptance-${suffix}"
data_dir="${work_dir}/data"
results_dir="${work_dir}/results"
fixture_dir="${work_dir}/fixtures"
cookie_file="${results_dir}/cookies.txt"
upload_response="${results_dir}/upload.json"
overview_before="${results_dir}/overview-before.json"
overview_after="${results_dir}/overview-after.json"
container_log="${results_dir}/container.log"
result_file="${results_dir}/acceptance.json"
fixture_file="${fixture_dir}/tiny.webp"
password="ImageSilo-${suffix}-Acceptance-Password!"

case "$work_dir" in
  /var/tmp/*) ;;
  *) printf 'WORK_DIR must be below /var/tmp\n' >&2; exit 1 ;;
esac
case "$duration_seconds:$request_interval" in
  *[!0-9:]*|0:*|*:0) printf 'DURATION_SECONDS and REQUEST_INTERVAL must be positive integers\n' >&2; exit 1 ;;
esac

cleanup() {
  docker rm --force "$container" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

hash_stdin() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | awk '{print $1}'
  else
    shasum -a 256 | awk '{print $1}'
  fi
}

json_field() {
  local file="$1"
  local key="$2"
  python3 -c 'import json, sys; print(json.load(open(sys.argv[1], encoding="utf-8"))[sys.argv[2]])' "$file" "$key"
}

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

mkdir -p "$data_dir" "$results_dir" "$fixture_dir"
if find "$data_dir" -mindepth 1 -print -quit | grep -q .; then
  printf 'WORK_DIR data directory must be empty: %s\n' "$data_dir" >&2
  exit 1
fi
printf '%s' 'UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA' | base64 --decode >"$fixture_file"

docker run --rm \
  --cpus "$cpu_limit" \
  --memory "$memory_limit" \
  --pids-limit "$pids_limit" \
  --user 0:0 \
  --entrypoint sh \
  --volume "${data_dir}:/data" \
  "$image" -c 'mkdir -p /data/db /data/images /data/cache/thumbnails /data/tmp && chown -R 10001:10001 /data'

printf '%s\n' "$password" | docker run --rm --interactive \
  --cpus "$cpu_limit" \
  --memory "$memory_limit" \
  --pids-limit "$pids_limit" \
  --volume "${data_dir}:/data" \
  "$image" admin create --email admin@example.com --password-stdin

docker run --detach \
  --name "$container" \
  --cpus "$cpu_limit" \
  --memory "$memory_limit" \
  --pids-limit "$pids_limit" \
  --publish "127.0.0.1:${port}:8080" \
  --env IMAGESILO_COOKIE_SECURE=false \
  --env IMAGESILO_PROCESSING_CONCURRENCY=1 \
  --volume "${data_dir}:/data" \
  "$image" >/dev/null

base_url="http://127.0.0.1:${port}"
wait_ready "$base_url"
curl --fail --silent --show-error \
  --cookie-jar "$cookie_file" \
  --header 'Content-Type: application/json' \
  --data "{\"email\":\"admin@example.com\",\"password\":\"${password}\"}" \
  "${base_url}/api/v1/auth/login" >/dev/null
csrf_token="$(awk '$6 == "imagesilo_csrf" { print $7 }' "$cookie_file" | tail -n 1)"
test -n "$csrf_token"

curl --fail --silent --show-error \
  --cookie "$cookie_file" \
  --header "X-CSRF-Token: ${csrf_token}" \
  --form "file=@${fixture_file};filename=acceptance.webp;type=image/webp" \
  --form 'visibility=public' \
  "${base_url}/api/v1/images" >"$upload_response"
image_id="$(json_field "$upload_response" id)"
expected_hash="$(json_field "$upload_response" storedSha256)"
test -n "$image_id"
test "$expected_hash" = "$(curl --fail --silent --show-error "${base_url}/image/${image_id}" | hash_stdin)"

curl --fail --silent --show-error \
  --cookie "$cookie_file" \
  --header "X-CSRF-Token: ${csrf_token}" \
  --header 'Content-Type: application/json' \
  --data "{\"path\":\"/legacy/acceptance.webp\",\"imageId\":\"${image_id}\",\"source\":\"remote-acceptance\"}" \
  "${base_url}/api/v1/aliases" >/dev/null

curl --fail --silent --show-error --cookie "$cookie_file" "${base_url}/api/v1/overview" >"$overview_before"
goroutines_before="$(json_field "$overview_before" goroutines)"
fds_before="$(docker exec "$container" sh -c 'find /proc/1/fd -mindepth 1 -maxdepth 1 | wc -l')"
started_at="$(date +%s)"
iterations=0
while test "$(( $(date +%s) - started_at ))" -lt "$duration_seconds"; do
  curl --fail --silent --show-error "${base_url}/healthz" >/dev/null
  curl --fail --silent --show-error "${base_url}/readyz" >/dev/null
  test "$expected_hash" = "$(curl --fail --silent --show-error "${base_url}/image/${image_id}" | hash_stdin)"
  test "$expected_hash" = "$(curl --fail --silent --show-error "${base_url}/legacy/acceptance.webp" | hash_stdin)"
  iterations="$((iterations + 1))"
  sleep "$request_interval"
done

curl --fail --silent --show-error --cookie "$cookie_file" "${base_url}/api/v1/overview" >"$overview_after"
goroutines_after="$(json_field "$overview_after" goroutines)"
fds_after="$(docker exec "$container" sh -c 'find /proc/1/fd -mindepth 1 -maxdepth 1 | wc -l')"
temporary_files="$(docker exec "$container" sh -c 'find /data/tmp -type f | wc -l')"
memory_current="$(docker exec "$container" sh -c 'cat /sys/fs/cgroup/memory.current')"
memory_peak="$(docker exec "$container" sh -c 'cat /sys/fs/cgroup/memory.peak')"
memory_anon="$(docker exec "$container" sh -c "awk '\$1 == \"anon\" { print \$2 }' /sys/fs/cgroup/memory.stat")"
memory_file="$(docker exec "$container" sh -c "awk '\$1 == \"file\" { print \$2 }' /sys/fs/cgroup/memory.stat")"
test "$goroutines_after" -le "$((goroutines_before + 4))"
test "$fds_after" -le "$((fds_before + 8))"
test "$temporary_files" = "0"

docker stop --timeout 15 "$container" >/dev/null
test "$(docker inspect --format '{{.State.ExitCode}}' "$container")" = "0"
docker logs "$container" >"$container_log" 2>&1
grep -q 'shutdown requested' "$container_log"

python3 - \
  "$result_file" "$image" "$work_dir" "$duration_seconds" "$iterations" "$cpu_limit" "$memory_limit" \
  "$goroutines_before" "$goroutines_after" "$fds_before" "$fds_after" \
  "$memory_current" "$memory_peak" "$memory_anon" "$memory_file" <<'PY'
import json
import sys

(
    result_file, image, work_dir, duration_seconds, iterations, cpu_limit, memory_limit,
    goroutines_before, goroutines_after, fds_before, fds_after,
    memory_current, memory_peak, memory_anon, memory_file,
) = sys.argv[1:]
with open(result_file, "w", encoding="utf-8") as output:
    json.dump({
        "image": image,
        "workDir": work_dir,
        "durationSeconds": int(duration_seconds),
        "iterations": int(iterations),
        "cpuLimit": float(cpu_limit),
        "memoryLimit": memory_limit,
        "goroutinesBefore": int(goroutines_before),
        "goroutinesAfter": int(goroutines_after),
        "fileDescriptorsBefore": int(fds_before),
        "fileDescriptorsAfter": int(fds_after),
        "memoryCurrentBytes": int(memory_current),
        "memoryPeakBytes": int(memory_peak),
        "memoryAnonBytes": int(memory_anon),
        "memoryFileBytes": int(memory_file),
    }, output, ensure_ascii=False, indent=2)
    output.write("\n")
PY

printf 'ImageSilo limited remote acceptance passed: result=%s data=%s\n' "$result_file" "$data_dir"
