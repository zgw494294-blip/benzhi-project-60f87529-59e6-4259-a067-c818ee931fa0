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
	file, err := os.OpenFile(s.eventPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return fmt.Errorf("打开事件日志: %w", err)
	}
	defer file.Close()
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

// truncateEventLog truncates the event log to the byte offset of the last
// complete frame, discarding any torn tail left by an interrupted write. It is
// a no-op when the log is already at or shorter than validOffset (for example
// when the file does not exist or no torn tail was present). The directory is
// synced so the truncated length survives a crash immediately after recovery.
func (s *Store) truncateEventLog(validOffset int64) error {
	info, err := os.Stat(s.eventPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("定位事件日志: %w", err)
	}
	if info.Size() <= validOffset {
		return nil
	}
	if err := os.Truncate(s.eventPath, validOffset); err != nil {
		return fmt.Errorf("截断事件日志: %w", err)
	}
	file, err := os.OpenFile(s.eventPath, os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("同步事件日志: %w", err)
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if syncErr != nil {
		return fmt.Errorf("同步事件日志: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("关闭事件日志: %w", closeErr)
	}
	directory, err := os.Open(s.directory)
	if err != nil {
		return fmt.Errorf("同步存储目录: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("同步存储目录: %w", err)
	}
	return nil
}

// readFrames reads and validates the length-prefixed JSON event frames stored
// in the log at path. It returns the fully validated frames and the byte offset
// in the file at which the stream of complete frames ended.
//
// A torn tail is tolerated: if the process was killed while writing the length
// prefix or the JSON payload of the next frame, the trailing partial bytes are
// ignored and validOffset marks the end of the last complete frame. Corruption
// of an already fully-written frame (bad length, malformed JSON, broken digest
// or sequence chain, invalid aggregate) is never tolerated and returns an
// error.
func readFrames(path string) (frames []EventFrame, validOffset int64, err error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("打开事件日志: %w", err)
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	frames = make([]EventFrame, 0)
	var previous string
	var expectedSequence int64 = 1
	var consumed int64
	for {
		var prefix [8]byte
		n, err := io.ReadFull(reader, prefix[:])
		consumed += int64(n)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			// A clean end of stream, or a partially written length prefix at
			// the very end of the log: the process was killed mid-frame. Stop
			// reading here; the caller truncates the log to validOffset.
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("事件长度前缀损坏: %w", err)
		}
		length := binary.BigEndian.Uint64(prefix[:])
		if length == 0 || length > maxFrameSize {
			return nil, 0, errors.New("事件长度不合法")
		}
		payload := make([]byte, int(length))
		n, err = io.ReadFull(reader, payload)
		consumed += int64(n)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				// The length prefix was complete, but the JSON payload was
				// never (fully) written: the process was killed mid-frame.
				// Stop reading here; the caller truncates the log to
				// validOffset.
				break
			}
			return nil, 0, fmt.Errorf("事件内容不完整: %w", err)
		}
		var frame EventFrame
		if err := json.Unmarshal(payload, &frame); err != nil {
			return nil, 0, fmt.Errorf("事件 JSON 损坏: %w", err)
		}
		if frame.SchemaVersion != schemaVersion || frame.Sequence != expectedSequence || frame.PreviousDigest != previous {
			return nil, 0, errors.New("事件序号或前序摘要链不连续")
		}
		digest, err := frameDigest(frame)
		if err != nil || digest != frame.Digest {
			return nil, 0, errors.New("事件摘要校验失败")
		}
		if frame.Aggregate == nil || frame.Aggregate.Dataset.ID != frame.DatasetID {
			return nil, 0, errors.New("事件聚合内容无效")
		}
		frames = append(frames, frame)
		previous = frame.Digest
		expectedSequence++
		validOffset = consumed
	}
	return frames, validOffset, nil
}
