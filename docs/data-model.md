# 数据模型

SQLite 初始模型的可执行事实来源是 [`../db/migrations/0001_initial.up.sql`](../db/migrations/0001_initial.up.sql)，默认设置由 `0002_seed_settings.up.sql` 写入，Session 绑定的 CSRF 哈希由 `0003_session_csrf.up.sql` 引入。

核心不变量：

- 管理员表通过 `singleton = 1` 的唯一约束保证最多一条记录。
- Session、Session 绑定的 CSRF Token 与 API Token 只保存 32 字节 SHA-256 哈希。
- 图片 SHA-256 不唯一；每次成功上传都创建新 UUID。
- 别名通过外键指向图片记录，图片删除时级联删除别名。
- 压缩和转换默认关闭，默认可见性为公开。
- 时间在 SQLite 中保存 Unix 秒，在 HTTP API 中返回 RFC 3339 UTC。
