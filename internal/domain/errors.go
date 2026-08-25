package domain

import "fmt"

type Error struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Field   string            `json:"field,omitempty"`
	Issues  []ValidationIssue `json:"issues,omitempty"`
}

// ValidationIssue 把批量请求中的错误定位到具体记录和字段。RecordIndex 使用从 0
// 开始的 JSON 数组下标，RecordNo 则是供工作台展示的从 1 开始序号。
type ValidationIssue struct {
	RecordIndex int    `json:"recordIndex"`
	RecordNo    int    `json:"recordNo"`
	Field       string `json:"field"`
	Code        string `json:"code"`
	Message     string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

func Invalid(field, message string) error {
	return &Error{Code: "validation_failed", Message: message, Field: field}
}

func InvalidBatch(issues []ValidationIssue) error {
	return &Error{Code: "validation_failed", Message: "批量片段校验未通过", Issues: issues}
}

func Conflict(message string) error {
	return &Error{Code: "state_conflict", Message: message}
}

func NotFound(kind, id string) error {
	return &Error{Code: "not_found", Message: fmt.Sprintf("%s %s 不存在", kind, id)}
}

func IsCode(err error, code string) bool {
	e, ok := err.(*Error)
	return ok && e.Code == code
}
