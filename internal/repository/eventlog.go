package repository

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

const maxFrameSize = 32 << 20

func frameDigest(frame EventFrame) (string, error) {
	frame.Digest = ""
	payload, err := json.Marshal(frame)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func (s *Store) appendFrame(frame *EventFrame) error {
	digest, err := frameDigest(*frame)
	if err != nil {
		return fmt.Errorf("计算事件摘要: %w", err)
	}
	frame.Digest = digest
	payload, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("序列化事件: %w", err)
	}
	if len(payload) > maxFrameSize {
		return errors.New("事件帧超出大小限制")
	}
	if s.eventFile == nil {
		file, err := os.OpenFile(s.eventPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
		if err != nil {
			return fmt.Errorf("打开事件日志: %w", err)
		}
		s.eventFile = file
	}
	file := s.eventFile
	var prefix [8]byte
	binary.BigEndian.PutUint64(prefix[:], uint64(len(payload)))
	if _, err := file.Write(prefix[:]); err != nil {
		return fmt.Errorf("写入事件长度: %w", err)
	}
	if _, err := file.Write(payload); err != nil {
		return fmt.Errorf("写入事件内容: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("同步事件日志: %w", err)
	}
	return nil
}

func readFrames(path string) ([]EventFrame, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("打开事件日志: %w", err)
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	frames := make([]EventFrame, 0)
	var previous string
	var expectedSequence int64 = 1
	for {
		var prefix [8]byte
		_, err := io.ReadFull(reader, prefix[:])
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("事件长度前缀损坏: %w", err)
		}
		length := binary.BigEndian.Uint64(prefix[:])
		if length == 0 || length > maxFrameSize {
			return nil, errors.New("事件长度不合法")
		}
		payload := make([]byte, int(length))
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, fmt.Errorf("事件内容不完整: %w", err)
		}
		var frame EventFrame
		if err := json.Unmarshal(payload, &frame); err != nil {
			return nil, fmt.Errorf("事件 JSON 损坏: %w", err)
		}
		if frame.SchemaVersion != schemaVersion || frame.Sequence != expectedSequence || frame.PreviousDigest != previous {
			return nil, errors.New("事件序号或前序摘要链不连续")
		}
		digest, err := frameDigest(frame)
		if err != nil || digest != frame.Digest {
			return nil, errors.New("事件摘要校验失败")
		}
		if frame.Aggregate == nil || frame.Aggregate.Dataset.ID != frame.DatasetID {
			return nil, errors.New("事件聚合内容无效")
		}
		frames = append(frames, frame)
		previous = frame.Digest
		expectedSequence++
	}
	return frames, nil
}
