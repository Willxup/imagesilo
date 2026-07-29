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
system_response="${TMPDIR:-/tmp}/${container}-system.json"
png_response="${TMPDIR:-/tmp}/${container}-png.json"
gif_response="${TMPDIR:-/tmp}/${container}-gif.json"
conversion_response="${TMPDIR:-/tmp}/${container}-conversion.json"
webp_response="${TMPDIR:-/tmp}/${container}-webp.json"
converted_webp="${TMPDIR:-/tmp}/${container}-converted.webp"
smoke_email="admin@example.com"
smoke_password="ImageSilo-${suffix}-Smoke-Password!"

cleanup() {
  docker rm --force "$container" "$restart_container" >/dev/null 2>&1 || true
  docker volume rm "$volume" >/dev/null 2>&1 || true
  rm -f "$cookie_file" "$upload_response" "$system_response" "$png_response" "$gif_response" \
    "$conversion_response" "$webp_response" "$converted_webp"
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

curl --fail --silent --show-error \
  --cookie "$cookie_file" \
  "${base_url}/api/v1/system" >"$system_response"
test "$(jq --raw-output '.vipsVersion' "$system_response")" = "8.18.4"
test "$(jq '.supportedFormats | length' "$system_response")" = "4"

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

go run ./tests/performance/image_stdout.go -format png -width 800 -height 600 |
  curl --fail --silent --show-error \
    --cookie "$cookie_file" \
    --header "X-CSRF-Token: ${csrf_token}" \
    --form 'file=@-;filename=container-smoke.png;type=image/png' \
    "${base_url}/api/v1/images" >"$png_response"
test "$(jq --raw-output '.mimeType' "$png_response")" = "image/png"
test "$(jq --raw-output '.sourceSha256' "$png_response")" = "$(jq --raw-output '.storedSha256' "$png_response")"

go run ./tests/performance/image_stdout.go -format gif -width 320 -height 240 |
  curl --fail --silent --show-error \
    --cookie "$cookie_file" \
    --header "X-CSRF-Token: ${csrf_token}" \
    --form 'file=@-;filename=container-smoke.gif;type=image/gif' \
    "${base_url}/api/v1/images" >"$gif_response"
test "$(jq --raw-output '.mimeType' "$gif_response")" = "image/gif"
gif_hash="$(jq --raw-output '.sourceSha256' "$gif_response")"
test "$gif_hash" = "$(jq --raw-output '.storedSha256' "$gif_response")"
test "$gif_hash" = "$(curl --fail --silent --show-error "${base_url}$(jq --raw-output '.standardUrl' "$gif_response")" | hash_stdin)"

curl --fail --silent --show-error \
  --cookie "$cookie_file" \
  --header "X-CSRF-Token: ${csrf_token}" \
  --header 'Content-Type: application/json' \
  --request PATCH \
  --data '{"compressionEnabled":false,"jpegQuality":85,"webpQuality":82,"pngCompressionLevel":6,"conversionEnabled":true,"conversionWebpQuality":82,"conversionWebpLossless":false}' \
  "${base_url}/api/v1/settings/processing" >/dev/null

go run ./tests/performance/image_stdout.go -format jpeg -width 800 -height 600 -quality 95 |
  curl --fail --silent --show-error \
    --cookie "$cookie_file" \
    --header "X-CSRF-Token: ${csrf_token}" \
    --form 'file=@-;filename=container-convert.jpg;type=image/jpeg' \
    "${base_url}/api/v1/images" >"$conversion_response"
test "$(jq --raw-output '.mimeType' "$conversion_response")" = "image/webp"
test "$(jq --raw-output '.processingSummary.action' "$conversion_response")" = "convert"
test "$(jq --raw-output '.sourceSha256' "$conversion_response")" != "$(jq --raw-output '.storedSha256' "$conversion_response")"
curl --fail --silent --show-error \
  --output "$converted_webp" \
  "${base_url}$(jq --raw-output '.standardUrl' "$conversion_response")"

curl --fail --silent --show-error \
  --cookie "$cookie_file" \
  --header "X-CSRF-Token: ${csrf_token}" \
  --header 'Content-Type: application/json' \
  --request PATCH \
  --data '{"compressionEnabled":false,"jpegQuality":85,"webpQuality":82,"pngCompressionLevel":6,"conversionEnabled":false,"conversionWebpQuality":82,"conversionWebpLossless":false}' \
  "${base_url}/api/v1/settings/processing" >/dev/null

curl --fail --silent --show-error \
  --cookie "$cookie_file" \
  --header "X-CSRF-Token: ${csrf_token}" \
  --form "file=@${converted_webp};filename=container-smoke.webp;type=image/webp" \
  "${base_url}/api/v1/images" >"$webp_response"
test "$(jq --raw-output '.mimeType' "$webp_response")" = "image/webp"
test "$(jq --raw-output '.sourceSha256' "$webp_response")" = "$(jq --raw-output '.storedSha256' "$webp_response")"

curl --fail --silent --show-error \
  --cookie "$cookie_file" \
  "${base_url}$(jq --raw-output '.thumbnailUrl' "$webp_response")" >/dev/null
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
