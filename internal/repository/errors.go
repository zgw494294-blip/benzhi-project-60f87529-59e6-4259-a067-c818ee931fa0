package repository

import "benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"

func versionConflict(expected, actual int64) error {
	return &domain.Error{Code: "version_conflict", Message: "expectedVersion 与当前版本不一致"}
}

func idempotencyConflict() error {
	return &domain.Error{Code: "idempotency_conflict", Message: "idempotencyKey 已用于其他命令"}
}
