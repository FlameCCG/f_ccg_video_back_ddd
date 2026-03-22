package video

import (
	"errors"
	"fmt"
)

// UploadSizePolicy 接收当次配置事实，统一单片、单文件及替换分片的累计上限。
type UploadSizePolicy struct{ MaxChunkMB, ChunkMB, FileMB int64 }

func (p UploadSizePolicy) chunkLimit() int64 {
	if p.MaxChunkMB > 0 {
		return p.MaxChunkMB
	}
	return p.ChunkMB
}

func (p UploadSizePolicy) IncomingChunk(size int64) error {
	const mebibyte int64 = 1024 * 1024
	if size < 0 {
		return errors.New("分块大小无效")
	}
	if limit := p.chunkLimit(); limit > 0 && size > limit*mebibyte {
		return fmt.Errorf("分块大小超过限制（最大 %dMB)", limit)
	}
	if p.FileMB > 0 && size > p.FileMB*mebibyte {
		return fmt.Errorf("文件大小超过限制（最大 %dMB)", p.FileMB)
	}
	return nil
}

func (p UploadSizePolicy) NeedsAggregate() bool { return p.FileMB > 0 }

func (p UploadSizePolicy) ReplacingChunk(total, current, size int64) error {
	const mebibyte int64 = 1024 * 1024
	if current > total {
		current = total
	}
	if p.FileMB > 0 && total-current+size > p.FileMB*mebibyte {
		return fmt.Errorf("文件大小超过限制（最大 %dMB)", p.FileMB)
	}
	return nil
}
