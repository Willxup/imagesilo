# 单张图片与历史路径导入

ImageSilo 的核心导入契约始终只有“一张合法图片 + 一个旧 URL path”。服务端不识别第三方图床格式，也不提供批量任务队列；任意来源都由外部脚本顺序枚举并调用同一个接口。

## 单张 curl

API Token 必须同时具备 `images:upload` 与 `aliases:write`；如果导入为私密并需要脚本自动下载校验，还必须具备 `images:read_private`：

```bash
curl --fail --show-error \
  --header "Authorization: Bearer $IMAGESILO_TOKEN" \
  --form 'file=@/absolute/path/example.jpg' \
  --form 'alias=/old/path/example.jpg' \
  --form 'visibility=public' \
  "$IMAGESILO_BASE_URL/api/v1/imports"
```

导入只验证格式、大小和像素并保存原始字节，不执行压缩或 WebP 转换。成功响应包含新 Image ID、标准 URL、SHA-256 和已创建的别名。

别名冲突返回 `409 Conflict`。图片记录与一个别名在同一个 SQLite 事务中提交；冲突或数据库失败不会留下新图片记录、正式文件或缩略图。

## 清单脚本

[import-manifest.sh](../scripts/import-manifest.sh) 使用 TSV 清单严格串行导入，并在每项成功后访问旧 URL、重新计算 SHA-256。它需要 `curl`、`jq`，以及 `sha256sum` 或 `shasum`。

清单格式：

```text
# 旧 URL path<TAB>本地图片路径
/i/2022/05/a.jpg	/absolute/source/a.jpg
/legacy/b.webp	/absolute/source/b.webp
```

执行：

```bash
IMAGESILO_BASE_URL=https://img.example.com \
IMAGESILO_TOKEN=ist_xxx \
scripts/import-manifest.sh \
  --manifest ./imports.tsv \
  --output ./imports-result.jsonl \
  --visibility public
```

结果文件已存在时脚本拒绝覆盖。出现冲突、HTTP 错误、响应 SHA 不一致或旧 URL 字节不一致时，脚本以非零状态退出，并保留逐项 JSONL 结果供筛选和重试。

## 迁移验收

- 清单输入数、成功数、冲突数和错误数可对账。
- 每个成功项的源文件、API 返回 SHA-256 和旧 URL 下载字节一致。
- 重复路径不会创建额外图片记录或正式文件。
- 迁移结束后手动执行系统巡检，并确认缺失文件、孤儿文件和清理失败均为零。
- 使用停止写入后的最终清单再执行一次正式迁移；不要把来源系统解析逻辑加入 ImageSilo 核心服务。
