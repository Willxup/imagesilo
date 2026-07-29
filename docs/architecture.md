# 架构基线

ImageSilo V1 是一个模块化单体：一个 Go 进程同时提供管理 API、React 静态资源和图片交付。SQLite 与正式图片文件是事实来源，内存索引是可从 SQLite 重建的派生数据。

依赖方向固定为：HTTP Handler → Feature Service → Repository；Service 可以调用 Storage 或 Processor，Repository 独占所属表的 SQL。图片交付 Handler 只读取内存索引和文件系统，不在请求热路径查询 SQLite。

V1 不引入微服务、Redis、外部队列、ORM、依赖注入容器或插件系统。只有实测瓶颈能够证明收益时才增加复杂度。

管理员密码使用 Argon2id（19 MiB 内存、2 次迭代、单线程、32 字节输出）。这是为单管理员登录选择的明确安全基线；缺失账号也验证一个预计算 dummy hash，保持时序接近但不在每次启动额外执行 Argon2id。

管理员 Session 和 API Token 都只在 SQLite 保存哈希，并在启动时加载到独立的小型内存索引。私密图片先读取图片交付索引中的可见性，再验证 Session 或 `images:read_private` Token；整个读取热路径只访问内存索引和文件系统。退出、改密、Token 吊销、过期清理和图片可见性修改都按“SQLite 提交成功后立即更新内存”的顺序执行。
