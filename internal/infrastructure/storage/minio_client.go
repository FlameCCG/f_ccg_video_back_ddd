package storage

import (
	"context"
	"encoding/json"
	"errors"
	"f_ccg_video_back_ddd/internal/config/site"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

// MinIOClientPool 缓存并复用 MinIO 客户端。
//
// 原实现使用包级变量 minioMu / minioClient / minioInitKey / minioPolicyOnce，
// 新架构改为可注入结构体：由 bootstrap 创建唯一实例，语义（按配置指纹缓存、
// 失败不缓存、桶策略只应用一次）与原实现一致。
type MinIOClientPool struct {
	mu         sync.Mutex
	client     *minio.Client
	initKey    string // endpoint|accessKey|secretKey|bucket|ssl，配置变更时重新初始化
	policyOnce sync.Once
	logger     *zap.Logger
}

// NewMinIOClientPool 创建 MinIO 客户端缓存。
func NewMinIOClientPool(logger *zap.Logger) *MinIOClientPool {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &MinIOClientPool{logger: logger}
}

// minioConfigKey 生成客户端缓存键；配置（含密钥）变更后需要重新建连。
func minioConfigKey(cfg *site.MinIO) string {
	return strings.Join([]string{
		strings.TrimSpace(cfg.Endpoint),
		strings.TrimSpace(cfg.AccessKey),
		strings.TrimSpace(cfg.SecretKey),
		strings.TrimSpace(cfg.Bucket),
		strconv.FormatBool(cfg.UseSSL),
	}, "\x00")
}

// Client 构建（并缓存）MinIO 客户端。
//
// 成功后按配置缓存；初始化失败不缓存，便于修正 accessKey/secretKey 后重试。
// 热更新导致 endpoint/凭证/bucket 变化时会重建客户端。
func (p *MinIOClientPool) Client(cfg *site.MinIO) (*minio.Client, error) {
	if cfg == nil {
		return nil, errors.New("未配置 MinIO 存储")
	}

	accessKey := strings.TrimSpace(cfg.AccessKey)
	secretKey := strings.TrimSpace(cfg.SecretKey)

	if accessKey == "" || secretKey == "" {
		return nil, errors.New("未配置 MinIO accessKey/secretKey（空凭证会导致 Access Denied）")
	}

	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("未设置 MinIO bucket 名称")
	}

	key := minioConfigKey(cfg)

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.client != nil && p.initKey == key {
		return p.client, nil
	}

	endpoint := cfg.Endpoint
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 MinIO 客户端失败: %w", err)
	}

	// 建桶检查属于存储驱动初始化的 root 场景，与请求生命周期无关。
	ctx := context.Background()

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("检查 MinIO bucket 失败: %w", err)
	}

	if !exists {
		if err = client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("创建 MinIO bucket 失败: %w", err)
		}
	}

	p.client = client
	p.initKey = key

	return p.client, nil
}

// applyPublicPolicy 为配置的公开前缀设置匿名只读桶策略（进程内只执行一次）。
func (p *MinIOClientPool) applyPublicPolicy(client *minio.Client, cfg *site.MinIO) {
	p.policyOnce.Do(func() {
		// MinIO policy 初始化是存储驱动后台维护任务，不绑定单个请求。
		ctx := context.Background()

		policy, err := buildPublicPolicy(cfg)
		if err != nil {
			p.logger.Warn("build minio policy failed", zap.String("event", "storage.minio_client.apply_public_policy.degraded"), zap.String("operation", "storage.minio_client.apply_public_policy"), zap.Error(err))

			return
		}

		if policy == "" {
			return
		}

		if err := client.SetBucketPolicy(ctx, cfg.Bucket, policy); err != nil {
			p.logger.Warn("set minio bucket policy failed", zap.String("event", "storage.minio_client.apply_public_policy.degraded"), zap.String("operation", "storage.minio_client.apply_public_policy"), zap.Error(err))
		} else {
			p.logger.Info("minio bucket policy applied", zap.String("event", "storage.minio_client.apply_public_policy.diagnostic"), zap.String("operation", "storage.minio_client.apply_public_policy"), zap.Strings("prefixes", cfg.PublicPrefixes))
		}
	})
}

func buildPublicPolicy(cfg *site.MinIO) (string, error) {
	if len(cfg.PublicPrefixes) == 0 {
		return "", nil
	}

	type statement struct {
		Effect    string         `json:"Effect"`
		Principal map[string]any `json:"Principal"`
		Action    []string       `json:"Action"`
		Resource  []string       `json:"Resource"`
	}

	type policyDoc struct {
		Version   string      `json:"Version"`
		Statement []statement `json:"Statement"`
	}

	resources := make([]string, 0, len(cfg.PublicPrefixes))

	for _, prefix := range cfg.PublicPrefixes {
		prefix = strings.Trim(prefix, "/")
		if prefix == "" {
			resources = append(resources, fmt.Sprintf("arn:aws:s3:::%s/*", cfg.Bucket))
		} else {
			resources = append(resources, fmt.Sprintf("arn:aws:s3:::%s/%s/*", cfg.Bucket, prefix))
		}
	}

	doc := policyDoc{
		Version: "2012-10-17",
		Statement: []statement{
			{
				Effect:    "Allow",
				Principal: map[string]any{"AWS": []string{"*"}},
				Action:    []string{"s3:GetObject"},
				Resource:  resources,
			},
		},
	}

	buf, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}

	return string(buf), nil
}
