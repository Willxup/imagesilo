# API Token 使用方式

API Token 必须在管理后台 `/admin/api-tokens` 创建。明文只在创建成功后显示一次；ImageSilo 只保存 SHA-256 哈希和用于识别的短前缀。

所有客户端都必须使用标准请求头：

```text
Authorization: Bearer ist_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

禁止把 Token 放进 `?token=`、`?key=`、`?api_key=` 或 `?access_token=`。图片接口会直接拒绝这类 URL，避免 Token 进入访问日志、浏览器历史和 Referer。

## curl 上传

Token 需要 `images:upload` Scope。省略 `visibility` 时使用系统默认值：

```bash
curl --fail --show-error \
  --header "Authorization: Bearer $IMAGESILO_TOKEN" \
  --form "file=@example.jpg;type=image/jpeg" \
  --form "visibility=private" \
  https://img.example.com/api/v1/images
```

响应中的 `standardUrl` 是稳定图片地址。私密图片需要管理员 Session，或具备 `images:read_private` Scope 的 Token：

```bash
curl --fail --show-error \
  --header "Authorization: Bearer $IMAGESILO_TOKEN" \
  https://img.example.com/image/019fadca-23c6-7664-81bb-10bd20cff14d
```

## curl 管理历史路径

Token 需要 `aliases:write` Scope。一次请求只创建一条路径映射，冲突返回 `409` 且不会覆盖原映射：

```bash
curl --fail-with-body --show-error \
  --header "Authorization: Bearer $IMAGESILO_TOKEN" \
  --header "Content-Type: application/json" \
  --data '{"path":"/i/2022/05/example.webp","imageId":"019fadca-23c6-7664-81bb-10bd20cff14d","source":"legacy-import"}' \
  https://img.example.com/api/v1/aliases
```

访问 `/i/2022/05/example.webp` 时服务直接返回目标图片字节，不发送 301/302。批量迁移由调用端循环这一单条接口；查询、解析和删除契约见 OpenAPI。

## PicGo

使用支持通用 multipart HTTP 请求的自定义上传插件，并填写：

- 请求方法：`POST`
- URL：`https://img.example.com/api/v1/images`
- 文件字段名：`file`
- Header：`Authorization: Bearer <具备 images:upload 的 Token>`
- 可选表单字段：`visibility=public` 或 `visibility=private`
- 返回 URL JSON 路径：`standardUrl`

Token 必须写在 Header 配置中，不能拼接到 URL。

## ShareX

将下面内容保存为自定义上传器配置并替换域名与 Token：

```json
{
  "Version": "16.1.0",
  "Name": "ImageSilo",
  "DestinationType": "ImageUploader",
  "RequestMethod": "POST",
  "RequestURL": "https://img.example.com/api/v1/images",
  "Headers": {
    "Authorization": "Bearer ist_replace_me"
  },
  "Body": "MultipartFormData",
  "Arguments": {
    "visibility": "public"
  },
  "FileFormName": "file",
  "URL": "{json:standardUrl}"
}
```

公开与私密读取、Scope、错误结构和全部请求字段的唯一契约来源是 [`../api/openapi.yaml`](../api/openapi.yaml)。
