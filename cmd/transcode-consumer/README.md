# 独立转码消费者

在项目根目录执行：

```sh
go run ./cmd/transcode-consumer -f config_dev.yaml
```

只消费 `kafka.topics.transcode` 指定的 topic，执行现有转码流程；使用配置中的
`kafka.groupID`，同组实例按分区分摊任务。启动失败返回非零退出码，Ctrl+C / SIGTERM
触发优雅停机。

此入口只连接转码所需的 MySQL、Redis、Kafka 和本地/MinIO 存储，不启动 HTTP、
WebSocket、River、视频调度和 Cron，也不执行初始化资源的运维任务。
转码进度、取消订阅、断点清扫和失败消息处理沿用现有消费者逻辑。

复用现有配置文件（包括其必填配置），需启用 Kafka 并配置转码 topic、消费组、
FFmpeg/FFprobe 及可访问的源文件存储。独立消费者始终承担转码角色，不受
`transcode.localConsumerDisabled` 影响；可在 HTTP 进程中开启该开关，将转码交给独立进程。
