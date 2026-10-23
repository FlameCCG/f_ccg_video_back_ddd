package search

import (
	"context"
	"errors"
	"fmt"

	"github.com/olivere/elastic/v7"
	"go.uber.org/zap"
)

// ErrIndexClientUnavailable 表示 ES 客户端未初始化，索引运维任务无法执行。
//
// 原 service/es_service 直接对 nil 客户端调用方法会 panic；此处改为返回错误，
// 让 CLI 以非 0 退出并给出可读原因。
var ErrIndexClientUnavailable = errors.New("elasticsearch client not initialized")

// IndexManager 负责搜索索引的创建/删除（原 service/es_service/index.go）。
//
// 只由启动流程与 CLI 运维任务（-es）调用，mapping JSON 由调用方 embed 后传入。
type IndexManager struct {
	client *elastic.Client
	logger *zap.Logger
}

// NewIndexManager 构造索引管理器（client 允许为 nil，方法将返回 ErrIndexClientUnavailable）。
func NewIndexManager(client *elastic.Client, logger *zap.Logger) *IndexManager {
	return &IndexManager{client: client, logger: logger}
}

// rootContext 返回索引运维任务使用的根上下文。
//
// ES 索引管理只由启动/CLI 运维任务调用，没有请求上下文。
func (m *IndexManager) rootContext() context.Context {
	return context.Background()
}

func (m *IndexManager) available() bool {
	return m != nil && m.client != nil
}

// CreateIndexV2 重建索引（存在则先删再建）.
func (m *IndexManager) CreateIndexV2(index, mapping string) error {
	exists, err := m.ExistsIndex(index)
	if err != nil {
		return err
	}

	if exists {
		if err := m.DeleteIndex(index); err != nil {
			return err
		}
	}

	return m.CreateIndex(index, mapping)
}

// CreateIndex 按 mapping 创建索引.
func (m *IndexManager) CreateIndex(index, mapping string) error {
	if !m.available() {
		return ErrIndexClientUnavailable
	}

	_, err := m.client.CreateIndex(index).BodyString(mapping).Do(m.rootContext())
	if err != nil {
		m.logger.Error("创建索引失败", zap.Error(err), zap.String("index", index))

		return fmt.Errorf("创建索引失败 %s: %w", index, err)
	}

	m.logger.Info("创建索引成功", zap.String("index", index))

	return nil
}

// ExistsIndex 判断索引是否存在.
func (m *IndexManager) ExistsIndex(index string) (bool, error) {
	if !m.available() {
		return false, ErrIndexClientUnavailable
	}

	exists, err := m.client.IndexExists(index).Do(m.rootContext())
	if err != nil {
		m.logger.Error("判断索引是否存在失败", zap.Error(err), zap.String("index", index))

		return false, fmt.Errorf("判断索引是否存在失败 %s: %w", index, err)
	}

	return exists, nil
}

// DeleteIndex 删除索引.
func (m *IndexManager) DeleteIndex(index string) error {
	if !m.available() {
		return ErrIndexClientUnavailable
	}

	_, err := m.client.DeleteIndex(index).Do(m.rootContext())
	if err != nil {
		m.logger.Error("删除索引失败", zap.Error(err), zap.String("index", index))

		return fmt.Errorf("删除索引失败 %s: %w", index, err)
	}

	m.logger.Info("删除索引成功", zap.String("index", index))

	return nil
}
