# MinIO 数据持久化与上传 Access Denied 排查

MinIO 使用 Docker 命名卷 `f_ccg_video_minio_data`，挂载到容器 `/data`。API 仍为 `localhost:9002`，控制台仍为 `localhost:9003`。Windows 上不要把 MinIO 的活动数据目录直接挂到 NTFS 项目目录；Docker Desktop 的 Linux 数据卷可避免经过 Windows 文件共享层执行对象提交时的重命名。

全新部署先执行 `docker volume create f_ccg_video_minio_data`，再启动 Compose。此卷声明为 external，由部署方管理其备份和生命周期。

## 2026-10-21 故障证据

- 后端在 08:23:38 和 08:27:36（America/Phoenix）记录 `上传分片失败: Access Denied.`。
- 容器以 root 运行，后端使用的凭证与容器 root 凭证一致。网站管理员绕过每日上传额度，但对象存储是独立服务。
- 原 `/data` 是 `E:\f_ccg_video_final\f_ccg_video_back_ddd\components\minio\data` 的 NTFS/9p 绑定挂载。磁盘当时仍有约 163 GiB 可用。
- 绕过 HTTP 后端，直接在容器内使用 root 凭证上传 10 MiB 分片，在索引 64 和 72 处分别失败，并非某个固定文件大小触发应用限制。
- 15:31:02 UTC 的存储追踪记录：`storage.RenameData`，目标 `videos/chunks/codex-probe-20261021/72.part`，错误 `file access denied`。
- 同一镜像和凭证换用 Linux 数据卷，连续 160 个 10 MiB 对象写入成功，共 1,677,721,600 字节。
- 实际迁移的 569 个底层文件逐一通过 SHA-256 校验；切换后的 272 个 S3 对象的 key、size、ETag 和修改时间与切换前一致。
- 切换后使用项目实际 `MinIOStorage` Go 驱动上传 160 个 10 MiB 分片、合并并完整读回，1,677,721,600 字节校验通过，耗时 37.086 秒。SHA-256 为 `d2822d4fa3b49c8dff1e8eb3f7b1381de8ef37397030d9a0af2a8d81b1cbab07`；诊断对象已删除。存储模块和视频应用模块的 Go 测试通过。

因此本次故障定位在 Windows 共享挂载的文件提交操作。现有证据不能进一步区分具体的 Windows 文件占用者、共享层错误或过滤驱动。Silo/MinIO 会把底层 `PrefixAccessDenied` 转为 S3 `AccessDenied`，仅凭客户端错误文案无法判断是 IAM 权限还是文件系统问题。

源码映射：[Silo API 错误处理](https://github.com/pgsty/minio/blob/master/cmd/api-errors.go)。Docker 的建议：[WSL 文件系统实践](https://docs.docker.com/desktop/features/wsl/best-practices/)、[数据卷](https://docs.docker.com/engine/storage/volumes/)。

## 已有绑定目录的迁移

更改卷配置本身不会导入原数据。已有部署应在维护窗口内按以下顺序操作：

1. 保存 S3 对象清单，包括 key、size、ETag；停止 MinIO，确保复制期间没有写入。
2. 创建空的目标卷 `f_ccg_video_minio_data`。
3. 使用当前 MinIO 镜像启动一次性复制容器：原数据目录只读挂载到 `/source`，目标卷挂载到 `/target`。确认 `/source/.minio.sys/format.json` 存在、目标为空，再用 `cp -a /source/. /target/` 复制完整目录，包括 `.minio.sys`。
4. 在两端均只读的状态下逐文件比较 SHA-256 和文件数量。校验失败时重启原部署，不切换。
5. 使用本仓库配置执行 `docker compose up -d --no-deps --pull never minio`，然后核对对象清单并执行上传、读取和合并验证。

原 `components/minio/data` 目录保留为切换前备份。切换后的新写入只进入数据卷；需要回退时必须先停服并同步切换后的数据，不能直接切回旧快照。备份和恢复应针对当前命名卷，不应再把旧目录当成活动存储。

## 再次出现错误时

后端的 `minio chunk write failed` 日志包含 bucket、key、size、S3 错误码、HTTP 状态和 requestID，可关联存储端追踪；不记录密钥或认证头。

先查看 `docker logs minio`，再使用已配置的 mc 别名执行 `mc admin trace --call storage,os --errors --json <alias>`。正常的对象探测会出现 `file not found`，应重点关注 `RenameData`、`CreateFile`、`WriteAll` 等写入操作的失败。

不要因 `AccessDenied` 直接扩大桶权限或公开写入。先确认是账号策略拒绝，还是服务器底层文件操作失败。
