-- 升级前停止旧版本调度器；新版本启动后会幂等迁移原 Redis ZSet 中的计划。
-- 也可通过 server -f <config.yaml> -db 执行同一模型的 AutoMigrate。
CREATE TABLE IF NOT EXISTS video_publish_schedules (
    video_id BIGINT UNSIGNED NOT NULL,
    publish_at DATETIME(3) NULL,
    PRIMARY KEY (video_id),
    KEY idx_video_publish_due (publish_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
