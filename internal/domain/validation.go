package domain

import (
	"encoding/hex"
	"strings"
	"time"
)

func NormalizeCodes(codes []string) ([]string, error) {
	seen := make(map[string]bool, len(codes))
	out := make([]string, 0, len(codes))
	for i, raw := range codes {
		code := strings.TrimSpace(raw)
		if code == "" {
			return nil, Invalid("taxonomyCodes", "分类编码不能为空")
		}
		if seen[code] {
			return nil, Invalid("taxonomyCodes", "分类编码不得重复")
		}
		if len(code) > 80 || i > 999 {
			return nil, Invalid("taxonomyCodes", "分类编码数量或长度超出限制")
		}
		seen[code] = true
		out = append(out, code)
	}
	if len(out) == 0 {
		return nil, Invalid("taxonomyCodes", "至少需要一个目标分类编码")
	}
	return out, nil
}

func ValidateDatasetInput(title, site, taxonomy string, from, to time.Time, devices []string) error {
	if strings.TrimSpace(title) == "" || len(title) > 200 {
		return Invalid("title", "数据集标题不能为空且不得超过 200 字符")
	}
	if strings.TrimSpace(site) == "" {
		return Invalid("siteCode", "采集地点编码不能为空")
	}
	if from.IsZero() || to.IsZero() || !to.After(from) {
		return Invalid("capturedTo", "采集结束时间必须晚于开始时间")
	}
	if strings.TrimSpace(taxonomy) == "" {
		return Invalid("taxonomyVersion", "分类体系版本不能为空")
	}
	if len(devices) == 0 {
		return Invalid("deviceCodes", "至少登记一台采集设备")
	}
	for _, device := range devices {
		if strings.TrimSpace(device) == "" {
			return Invalid("deviceCodes", "设备编码不能为空")
		}
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
