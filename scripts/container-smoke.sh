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
import_response="${TMPDIR:-/tmp}/${container}-import.json"
import_conflict_response="${TMPDIR:-/tmp}/${container}-import-conflict.json"
import_source="${TMPDIR:-/tmp}/${container}-import-source.jpg"
import_token_response="${TMPDIR:-/tmp}/${container}-import-token.json"
import_manifest="${TMPDIR:-/tmp}/${container}-import.tsv"
import_result="${TMPDIR:-/tmp}/${container}-import-result.jsonl"
overview_response="${TMPDIR:-/tmp}/${container}-overview.json"
converted_webp="${TMPDIR:-/tmp}/${container}-converted.webp"
smoke_email="admin@example.com"
smoke_password="ImageSilo-${suffix}-Smoke-Password!"

cleanup() {
  docker rm --force "$container" "$restart_container" >/dev/null 2>&1 || true
  docker volume rm "$volume" >/dev/null 2>&1 || true
  rm -f "$cookie_file" "$upload_response" "$system_response" "$png_response" "$gif_response" \
    "$conversion_response" "$webp_response" "$converted_webp"
  rm -f "$import_response" "$import_conflict_response" "$import_source" "$import_token_response" \
    "$import_manifest" "$import_result" "$overview_response"
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
test "$(jq '.processingConcurrency' "$system_response")" = "1"
test "$(jq '.maxTotalPixels' "$system_response")" = "16000000"

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

go run ./tests/performance/image_stdout.go -format jpeg -width 160 -height 120 -quality 90 >"$import_source"
import_source_hash="$(hash_stdin <"$import_source")"
curl --fail --silent --show-error \
  --cookie "$cookie_file" \
  --header "X-CSRF-Token: ${csrf_token}" \
  --header 'Content-Type: application/json' \
  --data '{"name":"container importer","scopes":["images:upload","aliases:write"]}' \
  "${base_url}/api/v1/api-tokens" >"$import_token_response"
import_token="$(jq --raw-output '.token' "$import_token_response")"
printf '/legacy/container-import.jpg\t%s\n' "$import_source" >"$import_manifest"
IMAGESILO_BASE_URL="$base_url" IMAGESILO_TOKEN="$import_token" \
  scripts/import-manifest.sh --manifest "$import_manifest" --output "$import_result" --visibility public
jq --raw-output '.response | fromjson' "$import_result" >"$import_response"
import_id="$(jq --raw-output '.imageId' "$import_response")"
test "$(jq --raw-output '.sha256' "$import_response")" = "$import_source_hash"

curl --fail --silent --show-error --cookie "$cookie_file" "${base_url}/api/v1/overview" >"$overview_response"
images_before_conflict="$(jq '.imageCount' "$overview_response")"
conflict_status="$(curl --silent --show-error \
  --output "$import_conflict_response" \
  --write-out '%{http_code}' \
  --cookie "$cookie_file" \
  --header "X-CSRF-Token: ${csrf_token}" \
  --form "file=@${import_source};filename=legacy-import.jpg;type=image/jpeg" \
  --form 'alias=/legacy/container-import.jpg' \
  --form 'visibility=public' \
  "${base_url}/api/v1/imports")"
test "$conflict_status" = "409"
curl --fail --silent --show-error --cookie "$cookie_file" "${base_url}/api/v1/overview" >"$overview_response"
test "$(jq '.imageCount' "$overview_response")" = "$images_before_conflict"

curl --fail --silent --show-error \
  --cookie "$cookie_file" \
  "${base_url}$(jq --raw-output '.thumbnailUrl' "$webp_response")" >/dev/null
test "$(docker inspect --format '{{.Config.User}}' "$container")" = "10001:10001"
test "$(docker inspect --format '{{.State.Health.Status}}' "$container")" = "healthy"
curl --fail --silent --show-error "${base_url}/admin/login" | grep -q '<div id="root"></div>'

docker exec "$container" rm "/data/images/${import_id}"

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
curl --fail --silent --show-error --cookie "$cookie_file" "${base_url}/api/v1/overview" >"$overview_response"
test "$(jq '.missingImageCount' "$overview_response")" = "1"
test "$(jq --raw-output --arg id "$import_id" '.missingImageIds | index($id) != null' "$overview_response")" = "true"
test "$(curl --silent --output /dev/null --write-out '%{http_code}' "${base_url}/legacy/container-import.jpg")" = "404"

printf 'ImageSilo container smoke passed: platform=%s image_id=%s sha256=%s\n' "$platform" "$image_id" "$expected_hash"
