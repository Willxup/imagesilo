#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
用法：
  IMAGESILO_BASE_URL=https://img.example.com \
  IMAGESILO_TOKEN=ist_xxx \
  scripts/import-manifest.sh --manifest ./imports.tsv --output ./imports-result.jsonl [--visibility public]

连接配置：
  IMAGESILO_BASE_URL  ImageSilo 根地址，不带结尾斜杠
  IMAGESILO_TOKEN     同时具备 images:upload 与 aliases:write Scope 的 API Token

运行参数：
  --manifest PATH    TSV 清单：每行“旧 URL path<TAB>本地图片路径”
  --output PATH      新建 JSONL 结果文件；为防止误覆盖，已存在时拒绝运行
  --visibility VALUE public 或 private，默认 public

脚本严格串行导入，并在每项成功后访问旧 URL、重新计算 SHA-256。
EOF
}

manifest=""
output=""
visibility="public"
while (($# > 0)); do
  case "$1" in
    --manifest)
      manifest="${2:-}"
      shift 2
      ;;
    --output)
      output="${2:-}"
      shift 2
      ;;
    --visibility)
      visibility="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf '未知参数：%s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

base_url="${IMAGESILO_BASE_URL:-}"
token="${IMAGESILO_TOKEN:-}"
if [[ -z "$base_url" || -z "$token" || -z "$manifest" || -z "$output" ]]; then
  printf '缺少连接配置或必填参数。\n' >&2
  usage >&2
  exit 2
fi
if [[ "$visibility" != "public" && "$visibility" != "private" ]]; then
  printf 'visibility 只能是 public 或 private。\n' >&2
  exit 2
fi
if [[ ! -f "$manifest" ]]; then
  printf '清单不存在：%s\n' "$manifest" >&2
  exit 2
fi
if [[ -e "$output" ]]; then
  printf '结果文件已存在，拒绝覆盖：%s\n' "$output" >&2
  exit 2
fi
for command_name in curl jq; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    printf '缺少命令：%s\n' "$command_name" >&2
    exit 2
  fi
done

base_url="${base_url%/}"
temporary_directory="$(mktemp -d "${TMPDIR:-/tmp}/imagesilo-import.XXXXXX")"
trap 'rm -rf "$temporary_directory"' EXIT INT TERM
header_file="$temporary_directory/authorization-header"
printf 'Authorization: Bearer %s\n' "$token" >"$header_file"
chmod 600 "$header_file"
: >"$output"

hash_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

successes=0
conflicts=0
errors=0
line_number=0
while IFS=$'\t' read -r alias_path image_path extra || [[ -n "${alias_path}${image_path}${extra}" ]]; do
  line_number=$((line_number + 1))
  if [[ -z "${alias_path// }" || "${alias_path:0:1}" == "#" ]]; then
    continue
  fi
  if [[ -z "$image_path" || -n "$extra" || ! -f "$image_path" ]]; then
    printf '第 %d 行格式错误或图片不存在。\n' "$line_number" >&2
    errors=$((errors + 1))
    jq -cn --argjson line "$line_number" --arg alias "$alias_path" --arg file "$image_path" \
      '{line:$line, alias:$alias, file:$file, status:"invalid_manifest"}' >>"$output"
    continue
  fi

  response_file="$temporary_directory/response-${line_number}.json"
  delivered_file="$temporary_directory/delivered-${line_number}"
  : >"$response_file"
  source_hash="$(hash_file "$image_path")"
  http_status="$(curl --silent --show-error \
    --output "$response_file" \
    --write-out '%{http_code}' \
    --header "@${header_file}" \
    --form "file=@${image_path}" \
    --form "alias=${alias_path}" \
    --form "visibility=${visibility}" \
    "${base_url}/api/v1/imports")" || http_status="000"

  if [[ "$http_status" == "201" ]]; then
    returned_hash="$(jq --raw-output '.sha256 // empty' "$response_file")"
    returned_alias="$(jq --raw-output '.alias.path // empty' "$response_file")"
    if [[ "$returned_hash" != "$source_hash" || "$returned_alias" != "$alias_path" ]]; then
      printf '第 %d 行响应校验失败：%s\n' "$line_number" "$alias_path" >&2
      errors=$((errors + 1))
      status="response_mismatch"
    elif ! curl --fail --silent --show-error --header "@${header_file}" --output "$delivered_file" "${base_url}${alias_path}"; then
      printf '第 %d 行旧 URL 访问失败：%s\n' "$line_number" "$alias_path" >&2
      errors=$((errors + 1))
      status="delivery_failed"
    elif [[ "$(hash_file "$delivered_file")" != "$source_hash" ]]; then
      printf '第 %d 行旧 URL 字节校验失败：%s\n' "$line_number" "$alias_path" >&2
      errors=$((errors + 1))
      status="hash_mismatch"
    else
      successes=$((successes + 1))
      status="imported"
      printf '已导入并验证：%s\n' "$alias_path"
    fi
  elif [[ "$http_status" == "409" ]]; then
    conflicts=$((conflicts + 1))
    status="conflict"
    printf '路径冲突：%s\n' "$alias_path" >&2
  else
    errors=$((errors + 1))
    status="error"
    printf '导入失败（HTTP %s）：%s\n' "$http_status" "$alias_path" >&2
  fi

  jq -cn \
    --argjson line "$line_number" \
    --arg alias "$alias_path" \
    --arg file "$image_path" \
    --arg sourceSha256 "$source_hash" \
    --arg status "$status" \
    --arg httpStatus "$http_status" \
    --rawfile response "$response_file" \
    '{line:$line, alias:$alias, file:$file, sourceSha256:$sourceSha256, status:$status, httpStatus:$httpStatus, response:$response}' \
    >>"$output"
done <"$manifest"

printf '导入完成：成功=%d，冲突=%d，错误=%d，结果=%s\n' "$successes" "$conflicts" "$errors" "$output"
if ((conflicts > 0 || errors > 0)); then
  exit 1
fi
