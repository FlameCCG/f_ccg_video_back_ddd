# 迁移逐文件核对表

> 唯一进度事实来源。每个 commit 必须同步更新本文件。
> `- [ ]` = pending，`- [x] ` = done。验收要求：0 个 pending。
> 箭头右侧为目标落点（规划值，落地后校正为实际路径）。

统计：共 577 个待迁移条目，其中 .go 文件 532 个（含 _test.go 68 个），资产/文档 45 个。

## (根目录)

- [x] `.gitignore` → 仓库根目录（原样复制）
- [x] `.golangci.yml` → 仓库根目录（原样复制）
- [x] `config.example.yaml` → 仓库根目录（原样复制）
- [x] `config_dev.yaml` → 仓库根目录（原样复制）
- [x] `docker-compose.yaml` → 仓库根目录（原样复制）
- [x] `go.mod` → 仓库根目录（原样复制）
- [x] `go.sum` → 仓库根目录（原样复制）
- [ ] `main.go` → cmd/server

## api

- [ ] `api/enter.go` → interfaces/http/router

## api/ai_api

- [ ] `api/ai_api/assets.go` → interfaces/http/handler/ai
- [ ] `api/ai_api/enter.go` → interfaces/http/handler/ai
- [ ] `api/ai_api/helpers.go` → interfaces/http/handler/ai
- [ ] `api/ai_api/images.go` → interfaces/http/handler/ai
- [ ] `api/ai_api/options.go` → interfaces/http/handler/ai
- [ ] `api/ai_api/responses.go` → interfaces/http/handler/ai
- [ ] `api/ai_api/videos.go` → interfaces/http/handler/ai

## api/banner_api

- [ ] `api/banner_api/admin_create.go` → interfaces/http/handler/contentops
- [ ] `api/banner_api/admin_delete.go` → interfaces/http/handler/contentops
- [ ] `api/banner_api/admin_list.go` → interfaces/http/handler/contentops
- [ ] `api/banner_api/admin_update.go` → interfaces/http/handler/contentops
- [ ] `api/banner_api/enter.go` → interfaces/http/handler/contentops
- [ ] `api/banner_api/helper.go` → interfaces/http/handler/contentops
- [ ] `api/banner_api/list.go` → interfaces/http/handler/contentops

## api/captcha_api

- [ ] `api/captcha_api/click_captcha.go` → interfaces/http/handler/identity
- [ ] `api/captcha_api/email_captcha.go` → interfaces/http/handler/identity
- [ ] `api/captcha_api/email_captcha_test.go` → interfaces/http/handler/identity
- [ ] `api/captcha_api/enter.go` → interfaces/http/handler/identity
- [ ] `api/captcha_api/graphics_text_captcha.go` → interfaces/http/handler/identity
- [ ] `api/captcha_api/slide_captcha.go` → interfaces/http/handler/identity

## api/chat_api

- [ ] `api/chat_api/conversations.go` → interfaces/http/handler/chat
- [ ] `api/chat_api/create_conversation.go` → interfaces/http/handler/chat
- [ ] `api/chat_api/delete_conversation.go` → interfaces/http/handler/chat
- [ ] `api/chat_api/enter.go` → interfaces/http/handler/chat
- [ ] `api/chat_api/messages.go` → interfaces/http/handler/chat
- [ ] `api/chat_api/read.go` → interfaces/http/handler/chat
- [ ] `api/chat_api/ws.go` → interfaces/http/handler/chat

## api/comment_api

- [ ] `api/comment_api/admin_delete.go` → interfaces/http/handler/comment
- [ ] `api/comment_api/admin_list.go` → interfaces/http/handler/comment
- [ ] `api/comment_api/create.go` → interfaces/http/handler/comment
- [ ] `api/comment_api/creator_list.go` → interfaces/http/handler/comment
- [ ] `api/comment_api/delete.go` → interfaces/http/handler/comment
- [ ] `api/comment_api/enter.go` → interfaces/http/handler/comment
- [ ] `api/comment_api/like.go` → interfaces/http/handler/comment
- [ ] `api/comment_api/list.go` → interfaces/http/handler/comment
- [ ] `api/comment_api/pin.go` → interfaces/http/handler/comment
- [ ] `api/comment_api/replies.go` → interfaces/http/handler/comment

## api/dynamic_api

- [ ] `api/dynamic_api/admin_delete.go` → interfaces/http/handler/dynamic
- [ ] `api/dynamic_api/admin_list.go` → interfaces/http/handler/dynamic
- [ ] `api/dynamic_api/counts.go` → interfaces/http/handler/dynamic
- [ ] `api/dynamic_api/create.go` → interfaces/http/handler/dynamic
- [ ] `api/dynamic_api/delete.go` → interfaces/http/handler/dynamic
- [ ] `api/dynamic_api/enter.go` → interfaces/http/handler/dynamic
- [ ] `api/dynamic_api/follows.go` → interfaces/http/handler/dynamic
- [ ] `api/dynamic_api/like.go` → interfaces/http/handler/dynamic
- [ ] `api/dynamic_api/list.go` → interfaces/http/handler/dynamic
- [ ] `api/dynamic_api/pin.go` → interfaces/http/handler/dynamic
- [ ] `api/dynamic_api/read.go` → interfaces/http/handler/dynamic

## api/notification_api

- [ ] `api/notification_api/admin_list.go` → interfaces/http/handler/notification
- [ ] `api/notification_api/admin_update.go` → interfaces/http/handler/notification
- [ ] `api/notification_api/counts.go` → interfaces/http/handler/notification
- [ ] `api/notification_api/create.go` → interfaces/http/handler/notification
- [ ] `api/notification_api/delete.go` → interfaces/http/handler/notification
- [ ] `api/notification_api/enter.go` → interfaces/http/handler/notification
- [ ] `api/notification_api/list.go` → interfaces/http/handler/notification
- [ ] `api/notification_api/read.go` → interfaces/http/handler/notification

## api/rbac_api

- [ ] `api/rbac_api/add_permission.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/assign_menu.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/assign_role.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/create_menu.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/create_role.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/delete_menu.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/delete_role.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/enter.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/menu_list.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/permission_helpers.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/permission_helpers_test.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/remove_menu.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/remove_permission.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/remove_role.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/resources.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/role_inherit.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/role_inherit_helpers.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/role_inherit_helpers_test.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/role_inherit_list.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/role_inherit_tree.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/role_list.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/role_menu_helper.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/role_menus.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/role_permissions.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/update_menu.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/update_role.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/update_role_permissions.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/update_user_roles.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/user_menus.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/user_permissions.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/user_roles.go` → interfaces/http/handler/rbac
- [ ] `api/rbac_api/wildcard_list.go` → interfaces/http/handler/rbac

## api/site_api

- [ ] `api/site_api/admin_config.go` → interfaces/http/handler/contentops
- [ ] `api/site_api/admin_config_test.go` → interfaces/http/handler/contentops
- [ ] `api/site_api/admin_cors_normalize_test.go` → interfaces/http/handler/contentops
- [ ] `api/site_api/admin_stats.go` → interfaces/http/handler/contentops
- [ ] `api/site_api/enter.go` → interfaces/http/handler/contentops
- [ ] `api/site_api/github_url.go` → interfaces/http/handler/contentops
- [ ] `api/site_api/google_url.go` → interfaces/http/handler/contentops
- [ ] `api/site_api/google_url_test.go` → interfaces/http/handler/contentops
- [ ] `api/site_api/linuxdo_url.go` → interfaces/http/handler/contentops
- [ ] `api/site_api/oauth_state_test.go` → interfaces/http/handler/contentops
- [ ] `api/site_api/qq_url.go` → interfaces/http/handler/contentops
- [ ] `api/site_api/stat_heartbeat.go` → interfaces/http/handler/contentops
- [ ] `api/site_api/stat_touch.go` → interfaces/http/handler/contentops
- [ ] `api/site_api/x_url.go` → interfaces/http/handler/contentops

## api/social_api

- [ ] `api/social_api/enter.go` → interfaces/http/handler/social
- [ ] `api/social_api/fans_list.go` → interfaces/http/handler/social
- [ ] `api/social_api/focus_list.go` → interfaces/http/handler/social
- [ ] `api/social_api/focus_status.go` → interfaces/http/handler/social
- [ ] `api/social_api/focus_user.go` → interfaces/http/handler/social
- [ ] `api/social_api/unfocus_user.go` → interfaces/http/handler/social

## api/user_api

- [ ] `api/user_api/admin_ban.go` → interfaces/http/handler/identity
- [ ] `api/user_api/admin_ban_records.go` → interfaces/http/handler/identity
- [ ] `api/user_api/admin_info.go` → interfaces/http/handler/identity
- [ ] `api/user_api/admin_list.go` → interfaces/http/handler/identity
- [ ] `api/user_api/admin_pwd_login.go` → interfaces/http/handler/identity
- [ ] `api/user_api/admin_update_user_info.go` → interfaces/http/handler/identity
- [ ] `api/user_api/bind_email.go` → interfaces/http/handler/identity
- [ ] `api/user_api/change_password.go` → interfaces/http/handler/identity
- [ ] `api/user_api/change_password_test.go` → interfaces/http/handler/identity
- [ ] `api/user_api/coin_records.go` → interfaces/http/handler/identity
- [ ] `api/user_api/create_tag.go` → interfaces/http/handler/identity
- [ ] `api/user_api/creator_analytics.go` → interfaces/http/handler/identity
- [ ] `api/user_api/delete_video.go` → interfaces/http/handler/identity
- [ ] `api/user_api/enter.go` → interfaces/http/handler/identity
- [ ] `api/user_api/exp_records.go` → interfaces/http/handler/identity
- [ ] `api/user_api/github_login.go` → interfaces/http/handler/identity
- [ ] `api/user_api/google_login.go` → interfaces/http/handler/identity
- [ ] `api/user_api/linuxdo_login.go` → interfaces/http/handler/identity
- [ ] `api/user_api/login_async.go` → interfaces/http/handler/identity
- [ ] `api/user_api/login_ip_records.go` → interfaces/http/handler/identity
- [ ] `api/user_api/mention_suggest.go` → interfaces/http/handler/identity
- [ ] `api/user_api/oauth_email_test.go` → interfaces/http/handler/identity
- [ ] `api/user_api/oauth_login_helper.go` → interfaces/http/handler/identity
- [ ] `api/user_api/oauth_state_test.go` → interfaces/http/handler/identity
- [ ] `api/user_api/pwd_login.go` → interfaces/http/handler/identity
- [ ] `api/user_api/qq_login.go` → interfaces/http/handler/identity
- [ ] `api/user_api/recent_actions.go` → interfaces/http/handler/identity
- [ ] `api/user_api/refresh_token.go` → interfaces/http/handler/identity
- [ ] `api/user_api/register.go` → interfaces/http/handler/identity
- [ ] `api/user_api/reset_password.go` → interfaces/http/handler/identity
- [ ] `api/user_api/search_user.go` → interfaces/http/handler/identity
- [ ] `api/user_api/switch_role.go` → interfaces/http/handler/identity
- [ ] `api/user_api/update_user_info.go` → interfaces/http/handler/identity
- [ ] `api/user_api/upload_banner.go` → interfaces/http/handler/identity
- [ ] `api/user_api/user_banner_helper.go` → interfaces/http/handler/identity
- [ ] `api/user_api/user_banner_setting.go` → interfaces/http/handler/identity
- [ ] `api/user_api/user_banner_setting_test.go` → interfaces/http/handler/identity
- [ ] `api/user_api/user_conf.go` → interfaces/http/handler/identity
- [ ] `api/user_api/user_default_banner.go` → interfaces/http/handler/identity
- [ ] `api/user_api/user_detail.go` → interfaces/http/handler/identity
- [ ] `api/user_api/user_info.go` → interfaces/http/handler/identity
- [ ] `api/user_api/user_like_tags_helper.go` → interfaces/http/handler/identity
- [ ] `api/user_api/user_videos.go` → interfaces/http/handler/identity
- [ ] `api/user_api/x_login.go` → interfaces/http/handler/identity

## api/video_api

- [ ] `api/video_api/admin_delete.go` → interfaces/http/handler/video
- [ ] `api/video_api/admin_detail.go` → interfaces/http/handler/video
- [ ] `api/video_api/admin_list.go` → interfaces/http/handler/video
- [ ] `api/video_api/admin_report_handle.go` → interfaces/http/handler/video
- [ ] `api/video_api/admin_report_list.go` → interfaces/http/handler/video
- [ ] `api/video_api/admin_restore.go` → interfaces/http/handler/video
- [ ] `api/video_api/admin_review.go` → interfaces/http/handler/video
- [ ] `api/video_api/coin.go` → interfaces/http/handler/video
- [ ] `api/video_api/danmu_admin_delete.go` → interfaces/http/handler/video
- [ ] `api/video_api/danmu_admin_list.go` → interfaces/http/handler/video
- [ ] `api/video_api/danmu_creator_list.go` → interfaces/http/handler/video
- [ ] `api/video_api/danmu_delete.go` → interfaces/http/handler/video
- [ ] `api/video_api/danmu_history.go` → interfaces/http/handler/video
- [ ] `api/video_api/danmu_like.go` → interfaces/http/handler/video
- [ ] `api/video_api/danmu_report.go` → interfaces/http/handler/video
- [ ] `api/video_api/danmu_room_list.go` → interfaces/http/handler/video
- [ ] `api/video_api/danmu_send.go` → interfaces/http/handler/video
- [ ] `api/video_api/danmu_ws.go` → interfaces/http/handler/video
- [ ] `api/video_api/detail.go` → interfaces/http/handler/video
- [ ] `api/video_api/enter.go` → interfaces/http/handler/video
- [ ] `api/video_api/favorite.go` → interfaces/http/handler/video
- [ ] `api/video_api/folder_create.go` → interfaces/http/handler/video
- [ ] `api/video_api/folder_delete.go` → interfaces/http/handler/video
- [ ] `api/video_api/folder_list.go` → interfaces/http/handler/video
- [ ] `api/video_api/folder_videos.go` → interfaces/http/handler/video
- [ ] `api/video_api/helpers.go` → interfaces/http/handler/video
- [ ] `api/video_api/history.go` → interfaces/http/handler/video
- [ ] `api/video_api/home.go` → interfaces/http/handler/video
- [ ] `api/video_api/hot.go` → interfaces/http/handler/video
- [ ] `api/video_api/like.go` → interfaces/http/handler/video
- [ ] `api/video_api/partition_admin.go` → interfaces/http/handler/video
- [ ] `api/video_api/partitions.go` → interfaces/http/handler/video
- [ ] `api/video_api/publish.go` → interfaces/http/handler/video
- [ ] `api/video_api/publish_private_test.go` → interfaces/http/handler/video
- [ ] `api/video_api/publish_resource_test.go` → interfaces/http/handler/video
- [ ] `api/video_api/rank.go` → interfaces/http/handler/video
- [ ] `api/video_api/recommend.go` → interfaces/http/handler/video
- [ ] `api/video_api/report.go` → interfaces/http/handler/video
- [ ] `api/video_api/search_hot.go` → interfaces/http/handler/video
- [ ] `api/video_api/search_video.go` → interfaces/http/handler/video
- [ ] `api/video_api/transcode_progress.go` → interfaces/http/handler/video
- [ ] `api/video_api/triple.go` → interfaces/http/handler/video
- [ ] `api/video_api/update.go` → interfaces/http/handler/video
- [ ] `api/video_api/upload_chunk.go` → interfaces/http/handler/video
- [ ] `api/video_api/upload_complete.go` → interfaces/http/handler/video
- [ ] `api/video_api/upload_cover.go` → interfaces/http/handler/video
- [ ] `api/video_api/upload_cover_test.go` → interfaces/http/handler/video
- [ ] `api/video_api/upload_status.go` → interfaces/http/handler/video
- [ ] `api/video_api/view.go` → interfaces/http/handler/video

## conf

- [x] `conf/conf_ai.go` → internal/infrastructure/config
- [x] `conf/conf_email.go` → internal/infrastructure/config
- [x] `conf/conf_es.go` → internal/infrastructure/config
- [x] `conf/conf_jwt.go` → internal/infrastructure/config
- [x] `conf/conf_kafka.go` → internal/infrastructure/config
- [x] `conf/conf_mongo.go` → internal/infrastructure/config
- [x] `conf/conf_mysql.go` → internal/infrastructure/config
- [x] `conf/conf_redis.go` → internal/infrastructure/config
- [x] `conf/conf_river.go` → internal/infrastructure/config
- [x] `conf/conf_river_test.go` → internal/infrastructure/config
- [x] `conf/conf_server.go` → internal/infrastructure/config
- [x] `conf/conf_third_login.go` → internal/infrastructure/config
- [x] `conf/conf_transcode.go` → internal/infrastructure/config
- [x] `conf/conf_zap.go` → internal/infrastructure/config
- [x] `conf/enter.go` → internal/infrastructure/config
- [x] `conf/rbac_model.conf` → internal/infrastructure/config

## conf/site

- [x] `conf/site/contentReview.go` → internal/infrastructure/config
- [x] `conf/site/enter.go` → internal/infrastructure/config
- [x] `conf/site/login.go` → internal/infrastructure/config
- [x] `conf/site/register.go` → internal/infrastructure/config
- [x] `conf/site/storage.go` → internal/infrastructure/config

## conf/third_login

- [x] `conf/third_login/enter.go` → internal/infrastructure/config

## core

- [ ] `core/enter.go` → bootstrap + infrastructure
- [ ] `core/graceful_stop.go` → bootstrap + infrastructure
- [ ] `core/graceful_stop_test.go` → bootstrap + infrastructure
- [x] `core/init_casbin.go` → internal/infrastructure/auth/casbin.go
- [x] `core/init_conf.go` → internal/infrastructure/config/provider.go（+ provider_test.go）
- [x] `core/init_conf_test.go` → internal/infrastructure/config/provider.go（+ provider_test.go）
- [x] `core/init_db.go` → internal/infrastructure/persistence/mysql/mysql.go
- [x] `core/init_es.go` → internal/infrastructure/search/es.go
- [x] `core/init_ip_db.go` → internal/infrastructure/ipregion（searcher.go + resolver.go）
- [ ] `core/init_mongo.go` → bootstrap + infrastructure
- [x] `core/init_redis.go` → internal/infrastructure/cache/redis.go
- [ ] `core/init_river.go` → bootstrap + infrastructure
- [ ] `core/init_transcode_consumer.go` → bootstrap + infrastructure
- [ ] `core/init_transcode_consumer_test.go` → bootstrap + infrastructure
- [ ] `core/init_video_scheduler.go` → bootstrap + infrastructure
- [x] `core/init_zap.go` → internal/infrastructure/logger/logger.go + internal/interfaces/http/middlewares/zap_gin.go

## docs

- [ ] `docs/ai_text_image_video_docs_2026-04-10.md` → docs
- [ ] `docs/openapi.yaml` → docs
- [ ] `docs/project-architecture.md` → docs
- [ ] `docs/transcode-dash-architecture.md` → docs

## docs/openapi-md

- [ ] `docs/openapi-md/index.md` → docs

## docs/openapi-md/admin

- [ ] `docs/openapi-md/admin/01-admin-user-management.md` → docs
- [ ] `docs/openapi-md/admin/02-admin-video-management.md` → docs
- [ ] `docs/openapi-md/admin/03-admin-comment-management.md` → docs
- [ ] `docs/openapi-md/admin/04-admin-dynamic-management.md` → docs
- [ ] `docs/openapi-md/admin/05-admin-banner-management.md` → docs
- [ ] `docs/openapi-md/admin/06-admin-notification-management.md` → docs
- [ ] `docs/openapi-md/admin/07-admin-site-management.md` → docs
- [ ] `docs/openapi-md/admin/08-rbac-management.md` → docs
- [ ] `docs/openapi-md/admin/index.md` → docs

## docs/openapi-md/user

- [ ] `docs/openapi-md/user/01-user-management.md` → docs
- [ ] `docs/openapi-md/user/02-captcha-management.md` → docs
- [ ] `docs/openapi-md/user/03-site-management.md` → docs
- [ ] `docs/openapi-md/user/04-banner-management.md` → docs
- [ ] `docs/openapi-md/user/05-video-management.md` → docs
- [ ] `docs/openapi-md/user/06-comment-management.md` → docs
- [ ] `docs/openapi-md/user/07-social-management.md` → docs
- [ ] `docs/openapi-md/user/08-notification-management.md` → docs
- [ ] `docs/openapi-md/user/09-chat-management.md` → docs
- [ ] `docs/openapi-md/user/10-dynamic-management.md` → docs
- [ ] `docs/openapi-md/user/11-ai-management.md` → docs
- [ ] `docs/openapi-md/user/index.md` → docs

## flags

- [ ] `flags/enter.go` → interfaces/cli
- [ ] `flags/flag_banner.go` → interfaces/cli
- [ ] `flags/flag_db.go` → interfaces/cli
- [ ] `flags/flag_db_exit_test.go` → interfaces/cli
- [ ] `flags/flag_db_test.go` → interfaces/cli
- [ ] `flags/flag_es.go` → interfaces/cli
- [ ] `flags/flag_rbac.go` → interfaces/cli
- [ ] `flags/flag_resource.go` → interfaces/cli
- [ ] `flags/flag_su.go` → interfaces/cli
- [ ] `flags/flag_video_partitions.go` → interfaces/cli
- [ ] `flags/flag_video_partitions_test.go` → interfaces/cli

## flags/mappings

- [x] `flags/mappings/users_mapping.json` → internal/interfaces/cli/mappings
- [x] `flags/mappings/videos_mapping.json` → internal/interfaces/cli/mappings

## global

- [x] `global/config_test.go` → 已吸收：internal/infrastructure/config.Provider + wire 注入（测试并入 config/provider_test.go）
- [x] `global/enter.go` → 已吸收：internal/infrastructure/config.Provider + wire 注入（测试并入 config/provider_test.go）

## init_resource/images/\347\224\250\346\210\267\351\205\215\347\275\256\346\250\252\345\271

- [x] `init_resource/images/\347\224\250\346\210\267\351\205\215\347\275\256\346\250\252\345\271\205-1.png` → init_resource（资产原样复制）
- [x] `init_resource/images/\347\224\250\346\210\267\351\205\215\347\275\256\346\250\252\345\271\205-2.png` → init_resource（资产原样复制）
- [x] `init_resource/images/\347\224\250\346\210\267\351\205\215\347\275\256\346\250\252\345\271\205-3.png` → init_resource（资产原样复制）

## init_resource/images/\351\246\226\351\241\265\350\275\256\346\222

- [x] `init_resource/images/\351\246\226\351\241\265\350\275\256\346\222\255-1.png` → init_resource（资产原样复制）
- [x] `init_resource/images/\351\246\226\351\241\265\350\275\256\346\222\255-2.jpeg` → init_resource（资产原样复制）
- [x] `init_resource/images/\351\246\226\351\241\265\350\275\256\346\222\255-3.jpg` → init_resource（资产原样复制）
- [x] `init_resource/images/\351\246\226\351\241\265\350\275\256\346\222\255-4.jpg` → init_resource（资产原样复制）

## init_resource/sql

- [x] `init_resource/sql/video_partitions.sql` → init_resource（资产原样复制）

## models/enum

- [x] `models/enum/banner.go` → internal/domain/shared/enum
- [x] `models/enum/comment.go` → internal/domain/shared/enum
- [x] `models/enum/dynamic.go` → internal/domain/shared/enum
- [x] `models/enum/gender.go` → internal/domain/shared/enum
- [x] `models/enum/notification.go` → internal/domain/shared/enum
- [x] `models/enum/register_source.go` → internal/domain/shared/enum
- [x] `models/enum/report.go` → internal/domain/shared/enum
- [x] `models/enum/user.go` → internal/domain/shared/enum
- [x] `models/enum/video.go` → internal/domain/shared/enum

## models/mongoEntity

- [x] `models/mongoEntity/chat_conversation.go` → internal/infrastructure/persistence/mongopo
- [x] `models/mongoEntity/chat_message.go` → internal/infrastructure/persistence/mongopo

## models/mysqlEntity

- [x] `models/mysqlEntity/api_resource.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/banner.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/casbin_rule.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/comment.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/comment_like.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/danmu.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/danmu_like.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/danmu_report.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/dynamic.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/dynamic_like.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/dynamic_read.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/enter.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/favorite_folder.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/favorite_folder_video.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/menu.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/notification.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/role.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/role_menu.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/tag.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/user.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/user_ban_record.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/user_banner_default.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/user_coin_record.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/user_conf.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/user_exp_record.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/user_focus.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/user_login_record.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/user_role.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/video.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/video_coin.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/video_like.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/video_part.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/video_part_resource.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/video_partition.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/video_play_history.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/video_report.go` → internal/infrastructure/persistence/po
- [x] `models/mysqlEntity/video_resource.go` → internal/infrastructure/persistence/po

## pkg

- [ ] `pkg/enter.go` → infrastructure/storage

## pkg/casbins

- [x] `pkg/casbins/enter.go` → pkg（平移）

## pkg/coinutil

- [x] `pkg/coinutil/coin.go` → pkg（平移）

## pkg/constants

- [x] `pkg/constants/enter.go` → pkg（平移）

## pkg/ctxutil

- [x] `pkg/ctxutil/detached.go` → pkg（平移）
- [x] `pkg/ctxutil/detached_test.go` → pkg（平移）
- [x] `pkg/ctxutil/timeout.go` → pkg（平移）
- [x] `pkg/ctxutil/timeout_test.go` → pkg（平移）

## pkg/emailStore

- [x] `pkg/emailStore/enter.go` → pkg（平移）

## pkg/encrypt

- [x] `pkg/encrypt/enter.go` → pkg（平移）

## pkg/jwt

- [x] `pkg/jwt/enter.go` → pkg（平移）
- [x] `pkg/jwt/enter_test.go` → pkg（平移）

## pkg/middlewares

- [ ] `pkg/middlewares/auth_all.go` → interfaces/http/middlewares
- [ ] `pkg/middlewares/body_limit.go` → interfaces/http/middlewares
- [ ] `pkg/middlewares/casbin_auth.go` → interfaces/http/middlewares
- [ ] `pkg/middlewares/click_captcha_verify.go` → interfaces/http/middlewares
- [x] `pkg/middlewares/cors.go` → internal/interfaces/http/middlewares/cors.go
- [ ] `pkg/middlewares/email_captcha_verify.go` → interfaces/http/middlewares
- [ ] `pkg/middlewares/graphics_text_captcha_verify.go` → interfaces/http/middlewares
- [ ] `pkg/middlewares/jwt_auth.go` → interfaces/http/middlewares
- [ ] `pkg/middlewares/jwt_optional.go` → interfaces/http/middlewares
- [ ] `pkg/middlewares/params_bind.go` → interfaces/http/middlewares
- [ ] `pkg/middlewares/request_timeout.go` → interfaces/http/middlewares
- [ ] `pkg/middlewares/request_timeout_test.go` → interfaces/http/middlewares
- [ ] `pkg/middlewares/slide_captcha_verify.go` → interfaces/http/middlewares
- [ ] `pkg/middlewares/uid.go` → interfaces/http/middlewares
- [ ] `pkg/middlewares/user_ban_check.go` → interfaces/http/middlewares

## pkg/mq

- [ ] `pkg/mq/enter.go` → infrastructure/mq

## pkg/ratelimit

- [ ] `pkg/ratelimit/redis.go` → infrastructure/cache/ratelimit

## pkg/recursion

- [ ] `pkg/recursion/enter.go` → domain/rbac

## pkg/res

- [x] `pkg/res/enter.go` → pkg（平移）

## pkg/uploadcheck

- [x] `pkg/uploadcheck/image.go` → pkg（平移）

## pkg/utils

- [x] `pkg/utils/enter.go` → pkg（平移）
- [x] `pkg/utils/pagination.go` → pkg（平移）

## pkg/utils/computerUtils

- [x] `pkg/utils/computerUtils/base_info.go` → pkg（平移）

## pkg/utils/dateUtils

- [x] `pkg/utils/dateUtils/enter.go` → pkg（平移）

## pkg/utils/ipUtils

- [x] `pkg/utils/ipUtils/enter.go` → pkg（平移）
- [x] `pkg/utils/ipUtils/enter_test.go` → pkg（平移）

## pkg/utils/markdownUtils

- [x] `pkg/utils/markdownUtils/enter.go` → pkg（平移）

## pkg/validate

- [x] `pkg/validate/enter.go` → pkg（平移）

## pkg/wsorigin

- [ ] `pkg/wsorigin/enter.go` → interfaces/ws/wsorigin

## pkg/xdb

- [x] `pkg/xdb/ip2region.xdb` → pkg（平移）

## router

- [ ] `router/ai_router.go` → interfaces/http/router
- [ ] `router/banner_admin_router.go` → interfaces/http/router
- [ ] `router/banner_router.go` → interfaces/http/router
- [ ] `router/captcha_router.go` → interfaces/http/router
- [ ] `router/chat_router.go` → interfaces/http/router
- [ ] `router/comment_admin_router.go` → interfaces/http/router
- [ ] `router/comment_router.go` → interfaces/http/router
- [ ] `router/dynamic_admin_router.go` → interfaces/http/router
- [ ] `router/dynamic_router.go` → interfaces/http/router
- [ ] `router/enter.go` → interfaces/http/router
- [ ] `router/notification_admin_router.go` → interfaces/http/router
- [ ] `router/notification_router.go` → interfaces/http/router
- [ ] `router/rbac_router.go` → interfaces/http/router
- [ ] `router/site_admin_router.go` → interfaces/http/router
- [ ] `router/site_router.go` → interfaces/http/router
- [ ] `router/social_router.go` → interfaces/http/router
- [ ] `router/user_admin_router.go` → interfaces/http/router
- [ ] `router/user_router.go` → interfaces/http/router
- [ ] `router/user_router_test.go` → interfaces/http/router
- [ ] `router/video_admin_router.go` → interfaces/http/router
- [ ] `router/video_router.go` → interfaces/http/router

## service/ai_service

- [ ] `service/ai_service/ark_provider.go` → application/ai + infrastructure/external/ark
- [ ] `service/ai_service/enter.go` → application/ai + infrastructure/external/ark
- [ ] `service/ai_service/types.go` → application/ai + infrastructure/external/ark
- [ ] `service/ai_service/video_prompt.go` → application/ai + infrastructure/external/ark

## service/banner_service

- [ ] `service/banner_service/banner_service.go` → domain/contentops + application/contentops
- [ ] `service/banner_service/cache.go` → domain/contentops + application/contentops

## service/captcha_service

- [ ] `service/captcha_service/click_captcha.go` → infrastructure/captcha
- [ ] `service/captcha_service/slide_captcha.go` → infrastructure/captcha

## service/chat_service

- [ ] `service/chat_service/chat_service.go` → domain/chat + application/chat + interfaces/ws
- [ ] `service/chat_service/conversation_manage.go` → domain/chat + application/chat + interfaces/ws
- [ ] `service/chat_service/conversation_repo.go` → domain/chat + application/chat + interfaces/ws
- [ ] `service/chat_service/fields.go` → domain/chat + application/chat + interfaces/ws
- [ ] `service/chat_service/page_test.go` → domain/chat + application/chat + interfaces/ws
- [ ] `service/chat_service/send_limit.go` → domain/chat + application/chat + interfaces/ws
- [ ] `service/chat_service/ws_hub.go` → domain/chat + application/chat + interfaces/ws

## service/comment_service

- [ ] `service/comment_service/admin_service.go` → domain/comment + application/comment
- [ ] `service/comment_service/comment_service.go` → domain/comment + application/comment
- [ ] `service/comment_service/like_cache.go` → domain/comment + application/comment

## service/cron_service

- [ ] `service/cron_service/auto_unban_user.go` → infrastructure/scheduler
- [ ] `service/cron_service/decay_video_hot.go` → infrastructure/scheduler
- [ ] `service/cron_service/enter.go` → infrastructure/scheduler
- [ ] `service/cron_service/enter_test.go` → infrastructure/scheduler
- [ ] `service/cron_service/sync_cache_test.go` → infrastructure/scheduler
- [ ] `service/cron_service/sync_comment_like.go` → infrastructure/scheduler
- [ ] `service/cron_service/sync_video_stat.go` → infrastructure/scheduler

## service/dynamic_service

- [ ] `service/dynamic_service/admin_service.go` → domain/dynamic + application/dynamic
- [ ] `service/dynamic_service/cache.go` → domain/dynamic + application/dynamic
- [ ] `service/dynamic_service/dynamic_service.go` → domain/dynamic + application/dynamic
- [ ] `service/dynamic_service/feed_service.go` → domain/dynamic + application/dynamic
- [ ] `service/dynamic_service/follow_service.go` → domain/dynamic + application/dynamic
- [ ] `service/dynamic_service/like_service.go` → domain/dynamic + application/dynamic
- [ ] `service/dynamic_service/read_service.go` → domain/dynamic + application/dynamic

## service/email_service

- [ ] `service/email_service/enter.go` → infrastructure/external/email
- [ ] `service/email_service/send_emailCode.go` → infrastructure/external/email
- [ ] `service/email_service/send_emailCode_test.go` → infrastructure/external/email

## service/es_service

- [ ] `service/es_service/index.go` → infrastructure/search

## service/login_service

- [ ] `service/login_service/http_retry.go` → application/identity + infrastructure/external/oauth
- [ ] `service/login_service/http_retry_test.go` → application/identity + infrastructure/external/oauth
- [ ] `service/login_service/oauth_state.go` → application/identity + infrastructure/external/oauth

## service/login_service/github_service

- [ ] `service/login_service/github_service/enter.go` → application/identity + infrastructure/external/oauth

## service/login_service/google_service

- [ ] `service/login_service/google_service/enter.go` → application/identity + infrastructure/external/oauth

## service/login_service/linuxdo_service

- [ ] `service/login_service/linuxdo_service/enter.go` → application/identity + infrastructure/external/oauth

## service/login_service/qq_service

- [ ] `service/login_service/qq_service/enter.go` → application/identity + infrastructure/external/oauth

## service/login_service/x_service

- [ ] `service/login_service/x_service/enter.go` → application/identity + infrastructure/external/oauth
- [ ] `service/login_service/x_service/enter_test.go` → application/identity + infrastructure/external/oauth

## service/mq_service

- [ ] `service/mq_service/enter.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/ffmpeg_progress.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/ffmpeg_progress_test.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/kafka_consumer.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/kafka_consumer_test.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/kafka_driver.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/kafka_driver_test.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/transcode_bitrate_test.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/transcode_checkpoint.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/transcode_checkpoint_test.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/transcode_chunked.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/transcode_chunked_test.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/transcode_consumer.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/transcode_consumer_test.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/transcode_pipeline_coordinator.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/transcode_pipeline_coordinator_test.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/transcode_progress.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/transcode_resource_test.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/transcode_task_state_test.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/transcode_timeout.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/transcode_timeout_test.go` → domain/media + application/media + infrastructure/mq
- [ ] `service/mq_service/video_meta_cache.go` → domain/media + application/media + infrastructure/mq

## service/notification_service

- [ ] `service/notification_service/cache.go` → domain/notification + application/notification
- [ ] `service/notification_service/notification_service.go` → domain/notification + application/notification

## service/rbac_service

- [ ] `service/rbac_service/init_service.go` → domain/rbac + application/rbac
- [ ] `service/rbac_service/resource_sync.go` → domain/rbac + application/rbac

## service/river_service

- [ ] `service/river_service/master.go` → infrastructure/river
- [ ] `service/river_service/master_test.go` → infrastructure/river
- [ ] `service/river_service/river.go` → infrastructure/river
- [ ] `service/river_service/river_test.go` → infrastructure/river
- [ ] `service/river_service/sync.go` → infrastructure/river

## service/river_service/rule

- [x] `service/river_service/rule/enter.go` → internal/infrastructure/river/rule
- [x] `service/river_service/rule/enter_test.go` → internal/infrastructure/river/rule

## service/search_service

- [ ] `service/search_service/base.go` → domain/search + application/search + infrastructure/search
- [ ] `service/search_service/cache.go` → domain/search + application/search + infrastructure/search
- [ ] `service/search_service/enter.go` → domain/search + application/search + infrastructure/search
- [ ] `service/search_service/fallback.go` → domain/search + application/search + infrastructure/search
- [ ] `service/search_service/hot_keywords.go` → domain/search + application/search + infrastructure/search
- [ ] `service/search_service/mention.go` → domain/search + application/search + infrastructure/search
- [ ] `service/search_service/mention_test.go` → domain/search + application/search + infrastructure/search
- [ ] `service/search_service/search.go` → domain/search + application/search + infrastructure/search
- [ ] `service/search_service/search_test.go` → domain/search + application/search + infrastructure/search
- [ ] `service/search_service/suggest.go` → domain/search + application/search + infrastructure/search
- [ ] `service/search_service/suggest_test.go` → domain/search + application/search + infrastructure/search
- [ ] `service/search_service/user_sync.go` → domain/search + application/search + infrastructure/search
- [ ] `service/search_service/video_vector.go` → domain/search + application/search + infrastructure/search

## service/site_service

- [ ] `service/site_service/admin_stats_service.go` → domain/contentops + application/contentops
- [ ] `service/site_service/stat_service.go` → domain/contentops + application/contentops

## service/storage_service

- [ ] `service/storage_service/local_storage.go` → infrastructure/storage
- [ ] `service/storage_service/local_storage_test.go` → infrastructure/storage
- [ ] `service/storage_service/minio_storage.go` → infrastructure/storage
- [ ] `service/storage_service/progress.go` → infrastructure/storage
- [ ] `service/storage_service/safety.go` → infrastructure/storage
- [ ] `service/storage_service/storage.go` → infrastructure/storage

## service/transcode_cancel_service

- [ ] `service/transcode_cancel_service/coordinator.go` → domain/media + infrastructure/mq
- [ ] `service/transcode_cancel_service/coordinator_test.go` → domain/media + infrastructure/mq

## service/user_service

- [ ] `service/user_service/admin_list.go` → domain/identity + application/identity
- [ ] `service/user_service/ban_record_service.go` → domain/identity + application/identity
- [ ] `service/user_service/ban_service.go` → domain/identity + application/identity
- [ ] `service/user_service/coin_record_service.go` → domain/identity + application/identity
- [ ] `service/user_service/coin_service.go` → domain/identity + application/identity
- [ ] `service/user_service/creator_analytics_service.go` → domain/identity + application/identity
- [ ] `service/user_service/enter.go` → domain/identity + application/identity
- [ ] `service/user_service/exp_record_service.go` → domain/identity + application/identity
- [ ] `service/user_service/exp_service.go` → domain/identity + application/identity
- [ ] `service/user_service/exp_service_test.go` → domain/identity + application/identity
- [ ] `service/user_service/focus_service.go` → domain/identity + application/identity
- [ ] `service/user_service/login_record_service.go` → domain/identity + application/identity
- [ ] `service/user_service/status_cache.go` → domain/identity + application/identity

## service/video_service

- [ ] `service/video_service/admin_delete_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/admin_delete_service_test.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/admin_restore_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/admin_video_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/async_test.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/author_cache.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/danmu_admin_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/danmu_creator_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/danmu_delete.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/danmu_hub.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/danmu_like.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/danmu_room_list.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/danmu_room_list_test.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/danmu_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/dash_mpd.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/dash_mpd_test.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/delete_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/detail_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/detail_service_test.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/enter.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/feed_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/folder_cache.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/folder_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/history_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/hot_rank_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/hot_score.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/interact_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/like_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/like_service_test.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/partition_cache.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/partition_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/recommend_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/report_admin_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/report_handle_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/report_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/scheduler_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/scheduler_service_test.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/stats_cache.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/stats_cache_test.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/status_resolve.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/status_resolve_test.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/tag_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/test_fixtures_test.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/transcode_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/triple_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/triple_service_test.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/update_private_test.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/update_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/upload_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/upload_service_test.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/user_stat_cache.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/user_stat_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/user_video_service.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/user_video_service_test.go` → domain/video + application/video + infrastructure/persistence/video
- [ ] `service/video_service/vector_sync.go` → domain/video + application/video + infrastructure/persistence/video

---

## 未完成项备注（防止误判为已完成）

- `main.go`：`cmd/server/main.go` 已建立并可运行 App，但 CLI 运维任务
  （`-db`/`-vp`/`-br`/`-su`/`-es`/`-rs`/`-rbac`）的分派待 `internal/interfaces/cli` 落地后接线。
- `core/init_mongo.go`：Mongo 客户端已迁移到 `internal/infrastructure/persistence/mongodb`，
  但原实现在连接成功后顺带执行的聊天集合索引初始化（`chat_service.EnsureIndexes`）
  需在 chat 上下文落地时重新接线，届时才标记 done。
- `core/enter.go` / `core/graceful_stop.go` / `core/init_river.go` /
  `core/init_transcode_consumer.go` / `core/init_video_scheduler.go`：
  对应 `App.Start` / `App.Stop` 的后台组件序列，随各上下文后台组件落地后补齐。
