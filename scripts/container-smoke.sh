#!/usr/bin/env bash
set -euo pipefail

image="${IMAGE:-imagesilo:smoke}"
platform="${PLATFORM:-linux/amd64}"
port="${PORT:-18080}"
suffix="${SMOKE_SUFFIX:-local}"
container="imagesilo-smoke-${suffix}"
restart_container="${container}-restart"
volume="${container}-data"
cookie_file="${TMPDIR:-/tmp}/${container}.cookies"
upload_response="${TMPDIR:-/tmp}/${container}-upload.json"
smoke_email="admin@example.com"
smoke_password="ImageSilo-${suffix}-Smoke-Password!"

cleanup() {
  docker rm --force "$container" "$restart_container" >/dev/null 2>&1 || true
  docker volume rm "$volume" >/dev/null 2>&1 || true
  rm -f "$cookie_file" "$upload_response"
}
trap cleanup EXIT INT TERM

hash_stdin() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | awk '{print $1}'
  else
    shasum -a 256 | awk '{print $1}'
  fi
}

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

wait_healthy() {
  local name="$1"
  for _ in $(seq 1 60); do
    if test "$(docker inspect --format '{{.State.Health.Status}}' "$name" 2>/dev/null || true)" = "healthy"; then
      return 0
    fi
    sleep 1
  done
  docker inspect "$name" || true
  return 1
}

cleanup
docker volume create "$volume" >/dev/null
printf '%s\n' "$smoke_password" | docker run --rm --interactive \
  --platform "$platform" \
  --volume "${volume}:/data" \
  "$image" admin create --email "$smoke_email" --password-stdin

docker run --detach --rm \
  --platform "$platform" \
  --name "$container" \
  --publish "127.0.0.1:${port}:8080" \
  --env IMAGESILO_COOKIE_SECURE=false \
  --volume "${volume}:/data" \
  "$image" >/dev/null

base_url="http://127.0.0.1:${port}"
wait_ready "$base_url"
wait_healthy "$container"

curl --fail --silent --show-error \
  --cookie-jar "$cookie_file" \
  --header 'Content-Type: application/json' \
  --data "{\"email\":\"${smoke_email}\",\"password\":\"${smoke_password}\"}" \
  "${base_url}/api/v1/auth/login" >/dev/null

csrf_token="$(awk '$6 == "imagesilo_csrf" { print $7 }' "$cookie_file" | tail -n 1)"
test -n "$csrf_token"

go run ./tests/performance/jpeg_stdout.go -width 3000 -height 2000 -quality 90 |
  curl --fail --silent --show-error \
    --cookie "$cookie_file" \
    --header "X-CSRF-Token: ${csrf_token}" \
    --form 'file=@-;filename=container-smoke.jpg;type=image/jpeg' \
    "${base_url}/api/v1/images" >"$upload_response"

image_id="$(jq --raw-output '.id' "$upload_response")"
expected_hash="$(jq --raw-output '.storedSha256' "$upload_response")"
test -n "$image_id"
test "$expected_hash" != "null"

actual_hash="$(curl --fail --silent --show-error "${base_url}/image/${image_id}" | hash_stdin)"
test "$actual_hash" = "$expected_hash"
test "$(docker inspect --format '{{.Config.User}}' "$container")" = "10001:10001"
test "$(docker inspect --format '{{.State.Health.Status}}' "$container")" = "healthy"
curl --fail --silent --show-error "${base_url}/admin/login" | grep -q '<div id="root"></div>'

docker stop --timeout 15 "$container" >/dev/null
docker run --detach --rm \
  --platform "$platform" \
  --name "$restart_container" \
  --publish "127.0.0.1:${port}:8080" \
  --env IMAGESILO_COOKIE_SECURE=false \
  --volume "${volume}:/data" \
  "$image" >/dev/null
container="$restart_container"
wait_ready "$base_url"
wait_healthy "$container"

restarted_hash="$(curl --fail --silent --show-error "${base_url}/image/${image_id}" | hash_stdin)"
test "$restarted_hash" = "$expected_hash"

printf 'ImageSilo container smoke passed: platform=%s image_id=%s sha256=%s\n' "$platform" "$image_id" "$expected_hash"
