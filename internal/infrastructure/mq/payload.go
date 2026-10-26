package mq

import (
	"encoding/json"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"

	media "f_ccg_video_back_ddd/internal/domain/media"
)

// TranscodePayload 是转码任务的 Kafka 消息体（原 pkg/mq.TranscodePayload）。
//
// 这些 json tag 属于外部契约（线上积压消息与其它实例都按此结构解析），一个字都不能改。
type TranscodePayload struct {
	ResolutionList []uint16              `json:"resolution"`
	StorageType    enum.VideoStorageType `json:"storageType"`
	SourcePath     string                `json:"sourcePath"`
	OutputPath     string                `json:"outputPath"`
	FileHash       string                `json:"fileHash"`
	FileName       string                `json:"fileName"`
	VideoID        uint                  `json:"videoId"`
	VideoPartID    uint                  `json:"videoPartId,omitempty"`
}

// PayloadFromTask 把领域任务实体转成 Kafka 消息体。
func PayloadFromTask(task media.TranscodeTask) TranscodePayload {
	return TranscodePayload{
		ResolutionList: task.ResolutionList,
		StorageType:    task.StorageType,
		SourcePath:     task.SourcePath,
		OutputPath:     task.OutputPath,
		FileHash:       task.FileHash,
		FileName:       task.FileName,
		VideoID:        task.VideoID,
		VideoPartID:    task.VideoPartID,
	}
}

// Task 把 Kafka 消息体转成领域任务实体。
func (p TranscodePayload) Task() media.TranscodeTask {
	return media.TranscodeTask{
		ResolutionList: p.ResolutionList,
		StorageType:    p.StorageType,
		SourcePath:     p.SourcePath,
		OutputPath:     p.OutputPath,
		FileHash:       p.FileHash,
		FileName:       p.FileName,
		VideoID:        p.VideoID,
		VideoPartID:    p.VideoPartID,
	}
}

// decodeTranscodePayload 反序列化 Kafka 消息体。
func decodeTranscodePayload(payloadBytes []byte) (TranscodePayload, error) {
	var payload TranscodePayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return TranscodePayload{}, err
	}

	return payload, nil
}

// taskIdentityFromMessage 只解析消息中的身份字段，用于在完整反序列化失败时仍能定位任务。
func taskIdentityFromMessage(payloadBytes []byte) media.TaskKey {
	var identity struct {
		VideoID     uint `json:"videoId"`
		VideoPartID uint `json:"videoPartId"`
	}

	if err := json.Unmarshal(payloadBytes, &identity); err != nil {
		return media.TaskKey{}
	}

	return media.TaskKey{VideoID: identity.VideoID, PartID: identity.VideoPartID}
}
