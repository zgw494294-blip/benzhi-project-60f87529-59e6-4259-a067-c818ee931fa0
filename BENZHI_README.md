# BENZHI_README

基于 Go 实现的声境标注放行台 Web 项目，一款后端服务，完整实现生态声学录音数据集从编目、标注、复核整改、批准冻结到凭据发布与只读核验的浏览器工作台，并以带摘要校验链的本地事件日志和原子快照持久化。

## 项目说明
- 项目：benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0
- 项目用途：完整实现生态声学录音数据集从编目、标注、复核整改、批准冻结到凭据发布与只读核验的浏览器工作台，并以带摘要校验链的本地事件日志和原子快照持久化。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0-arm64 linux/arm64
docker run -it benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -selfcheck -addr=127.0.0.1:19081`
