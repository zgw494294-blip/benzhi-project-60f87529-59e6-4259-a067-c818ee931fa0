package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/application"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/httpui"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/repository"
)

type checkResult struct {
	DatasetID string          `json:"datasetId"`
	Version   int64           `json:"version"`
	Status    string          `json:"status"`
	Resource  json.RawMessage `json:"resource"`
}

func runSelfcheck(address string) error {
	directory, err := os.MkdirTemp("", "shengjing-selfcheck-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	store, err := repository.Open(directory)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &http.Server{Handler: httpui.New(application.NewService(store), logger), ReadHeaderTimeout: 3 * time.Second}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("自检监听 %s: %w", address, err)
	}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	baseURL := "http://" + listener.Addr().String()
	client := &http.Client{Timeout: 4 * time.Second}
	checkContext, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	flowErr := executeCheckFlow(checkContext, client, baseURL)
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	shutdownErr := server.Shutdown(shutdownContext)
	serveErr := <-serveErrors
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	if flowErr != nil {
		return flowErr
	}
	if shutdownErr != nil {
		return shutdownErr
	}
	fmt.Println("自检通过：已通过真实 HTTP 监听完成创建、编目、标注、送审、批准、冻结、发布及凭据核验")
	return nil
}

func executeCheckFlow(ctx context.Context, client *http.Client, baseURL string) error {
	now := time.Now().UTC().Truncate(time.Second)
	created, err := checkPost(ctx, client, baseURL+"/api/v1/datasets", map[string]any{
		"expectedVersion": 0, "idempotencyKey": "check-create", "title": "端到端自检声景",
		"siteCode": "CHECK-SITE", "capturedFrom": now.Add(-time.Hour), "capturedTo": now.Add(time.Hour),
		"taxonomyVersion": "check-v1", "taxonomyCodes": []string{"bird.check"}, "deviceCodes": []string{"REC-CHECK"},
	})
	if err != nil {
		return err
	}
	clipResult, err := checkPost(ctx, client, baseURL+"/api/v1/datasets/"+created.DatasetID+"/clips", map[string]any{
		"expectedVersion": created.Version, "idempotencyKey": "check-clip", "sourceName": "check.wav",
		"startedAt": now, "durationMs": 10000, "channelCount": 1,
		"sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "deviceCode": "REC-CHECK",
		"metadata": map[string]string{"habitat": "自检林地"},
	})
	if err != nil {
		return err
	}
	var clip struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(clipResult.Resource, &clip); err != nil || clip.ID == "" {
		return errors.New("自检未取得片段标识")
	}
	annotationResult, err := checkPost(ctx, client, baseURL+"/api/v1/datasets/"+created.DatasetID+"/annotations", map[string]any{
		"expectedVersion": clipResult.Version, "idempotencyKey": "check-annotation", "clipId": clip.ID,
		"startMs": 0, "endMs": 5000, "labelCode": "bird.check", "confidence": 0.95,
		"note": "自检标注", "createdBy": "selfcheck",
	})
	if err != nil {
		return err
	}
	submitted, err := checkPost(ctx, client, baseURL+"/api/v1/datasets/"+created.DatasetID+"/submit", map[string]any{"expectedVersion": annotationResult.Version, "idempotencyKey": "check-submit"})
	if err != nil {
		return err
	}
	approved, err := checkPost(ctx, client, baseURL+"/api/v1/datasets/"+created.DatasetID+"/approve", map[string]any{"expectedVersion": submitted.Version, "idempotencyKey": "check-approve", "reviewedBy": "selfcheck-reviewer"})
	if err != nil {
		return err
	}
	var preview struct {
		Digest      string `json:"digest"`
		BaseVersion int64  `json:"baseVersion"`
	}
	if err := checkGet(ctx, client, baseURL+"/api/v1/datasets/"+created.DatasetID+"/freeze/preview", &preview); err != nil {
		return err
	}
	if preview.Digest == "" || preview.BaseVersion != approved.Version {
		return errors.New("冻结清单预览无效")
	}
	frozen, err := checkPost(ctx, client, baseURL+"/api/v1/datasets/"+created.DatasetID+"/freeze", map[string]any{"expectedVersion": approved.Version, "idempotencyKey": "check-freeze", "previewDigest": preview.Digest})
	if err != nil {
		return err
	}
	released, err := checkPost(ctx, client, baseURL+"/api/v1/datasets/"+created.DatasetID+"/release", map[string]any{"expectedVersion": frozen.Version, "idempotencyKey": "check-release", "issuedBy": "selfcheck-owner"})
	if err != nil {
		return err
	}
	var credential struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(released.Resource, &credential); err != nil || credential.ID == "" {
		return errors.New("自检未取得发布凭据")
	}
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/credentials/"+credential.ID, nil)
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var verification struct {
		Verified bool `json:"verified"`
	}
	if err := json.NewDecoder(response.Body).Decode(&verification); err != nil || response.StatusCode != http.StatusOK || !verification.Verified {
		return errors.New("发布凭据核验失败")
	}
	return nil
}

func checkGet(ctx context.Context, client *http.Client, url string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		failure, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("自检请求 %s 返回 %d: %s", url, response.StatusCode, failure)
	}
	return json.NewDecoder(response.Body).Decode(target)
}

func checkPost(ctx context.Context, client *http.Client, url string, payload any) (checkResult, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return checkResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return checkResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return checkResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		failure, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return checkResult{}, fmt.Errorf("自检请求 %s 返回 %d: %s", url, response.StatusCode, failure)
	}
	var result checkResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return checkResult{}, err
	}
	return result, nil
}
