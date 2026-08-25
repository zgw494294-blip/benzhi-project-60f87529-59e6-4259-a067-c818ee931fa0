# 声境标注放行台

声境标注放行台是面向生态声学资料管理员、物种标注员和质量复核员的单流程浏览器工作台。它把野外录音资料从 `draft` 编目推进到 `in_review`、`remediation`、`approved`、`frozen` 和 `released`，最终生成带递增序号、稳定清单摘要和签发时间的不可变发布凭据。

浏览器页面和同源 JSON API 均由 Go 服务提供，不需要 Node.js 构建链。写操作使用 `expectedVersion` 防止并发覆盖，并使用 `idempotencyKey` 保证安全重试。本地持久化由长度前缀 JSON 事件帧日志和原子投影快照组成；事件带序号、前序摘要和当前摘要，投影损坏时会自动从事件日志恢复。

## 构建与运行

要求 Go 1.22 或更高版本。

```text
go build ./cmd/server
go run ./cmd/server -addr=127.0.0.1:19081 -data=./data
```

默认地址是 `127.0.0.1:19081`。也可以设置 `PORT` 为端口号，此时服务监听 `127.0.0.1:<PORT>`；显式 `-addr` 优先于默认值。服务拒绝空主机以及 `0.0.0.0`、`::` 这类全接口监听地址。

打开 `http://127.0.0.1:19081/` 后，可在页面中完成以下流程：

1. 创建数据集并维护采集时间窗、地点、设备和分类体系。
2. 单条或批量登记录音片段，并按来源、设备、时间和生境组合检索目录与完整性统计。
3. 添加标注修订、对比历史版本，或从指定历史修订继承后只调整必要字段。
4. 先执行无副作用送审预检，再处理自动一致性问题、人工问题与只追加整改结论轨迹。
5. 关闭全部阻断问题后批准，核对候选清单摘要后确认冻结并发布。
6. 从发布结果打开只读凭据核验页面。

## 测试与自检

运行全部回归测试：

```text
go test ./...
```

运行真实 HTTP 端到端自检：

```text
go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
```

自检会在给定地址建立真实监听，使用临时存储并通过 HTTP 完成创建、编目、标注、送审、批准、冻结、发布和凭据核验，随后优雅关闭并自行退出。

## 主要接口

- `GET /`：浏览器工作台。
- `GET /healthz`：健康检查。
- `POST /api/v1/datasets`：创建数据集。
- `GET /api/v1/datasets`、`GET /api/v1/datasets/{datasetId}`：列表与工作台详情。
- `POST /api/v1/datasets/{datasetId}/metadata`：维护 draft 登记信息。
- `POST /api/v1/datasets/{datasetId}/clips`：登记录音片段。
- `POST /api/v1/datasets/{datasetId}/clips/batch`：原子批量登记片段并返回逐项校验问题。
- `GET /api/v1/datasets/{datasetId}/clips`：组合筛选、稳定分页并读取当前筛选统计。
- `POST /api/v1/datasets/{datasetId}/annotations`：添加标注修订，或通过 `sourceRevisionId` 继承历史修订。
- `GET /api/v1/datasets/{datasetId}/annotations/compare`：对比同一片段的两个历史修订。
- `GET /api/v1/datasets/{datasetId}/preflight`：对当前 draft 版本执行只读送审预检。
- `POST /api/v1/datasets/{datasetId}/submit`：送审并运行一致性校验。
- `POST /api/v1/datasets/{datasetId}/issues`：登记人工问题。
- `POST /api/v1/datasets/{datasetId}/issues/{issueId}/resolve`：登记整改证据和复核结论。
- `POST /api/v1/datasets/{datasetId}/issues/{issueId}/reopen`：保留结论轨迹并重开已关闭问题。
- `POST /api/v1/datasets/{datasetId}/approve`：关闭阻断项后批准。
- `GET /api/v1/datasets/{datasetId}/freeze/preview`：只读生成候选清单、统计与摘要。
- `POST /api/v1/datasets/{datasetId}/freeze`：携带 `previewDigest` 确认并生成不可编辑冻结清单。
- `POST /api/v1/datasets/{datasetId}/release`：签发发布凭据。
- `GET /api/v1/datasets/{datasetId}/manifest`：读取冻结清单。
- `GET /api/v1/datasets/{datasetId}/events`：读取带摘要链的业务轨迹。
- `GET /api/v1/credentials/{credentialId}`、`GET /verify/{credentialId}`：JSON 与 HTML 只读核验视图。

所有 JSON 写接口限制请求体为 1 MiB、拒绝未知字段，并返回稳定的 `error.code`、中文说明和可选字段位置。
