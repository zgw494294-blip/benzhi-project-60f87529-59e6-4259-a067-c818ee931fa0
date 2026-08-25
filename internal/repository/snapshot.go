package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func stateDigest(state persistedState) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func (s *Store) restore() error {
	frames, err := readFrames(s.eventPath)
	if err != nil {
		return err
	}
	fromSnapshot, snapshotErr := s.readSnapshot()
	if snapshotErr == nil && fromSnapshot.Sequence == int64(len(frames)) {
		if len(frames) == 0 || fromSnapshot.LastDigest == frames[len(frames)-1].Digest {
			s.state = fromSnapshot
			return nil
		}
	}
	s.state = emptyState()
	for _, frame := range frames {
		s.applyFrame(frame)
	}
	if len(frames) > 0 || snapshotErr != nil {
		if err := s.writeSnapshot(); err != nil {
			return fmt.Errorf("重建投影: %w", err)
		}
	}
	return nil
}

func (s *Store) readSnapshot() (persistedState, error) {
	payload, err := os.ReadFile(s.snapshotPath)
	if err != nil {
		return persistedState{}, err
	}
	var envelope snapshotEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return persistedState{}, fmt.Errorf("解析投影: %w", err)
	}
	if envelope.SchemaVersion != schemaVersion {
		return persistedState{}, errors.New("投影 schemaVersion 不受支持")
	}
	digest, err := stateDigest(envelope.State)
	if err != nil || digest != envelope.Digest {
		return persistedState{}, errors.New("投影摘要校验失败")
	}
	if envelope.State.Datasets == nil || envelope.State.Idempotency == nil {
		return persistedState{}, errors.New("投影缺少必要映射")
	}
	return envelope.State, nil
}

func (s *Store) writeSnapshot() error {
	digest, err := stateDigest(s.state)
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(snapshotEnvelope{SchemaVersion: schemaVersion, State: s.state, Digest: digest}, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(s.directory, ".projection-*.tmp")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	cleanup := func() { _ = os.Remove(temporaryName) }
	defer cleanup()
	if err := temporary.Chmod(0o640); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(payload); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, s.snapshotPath); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(s.snapshotPath))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
