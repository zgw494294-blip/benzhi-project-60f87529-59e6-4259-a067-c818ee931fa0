package httpui

import (
	"html/template"
	"net/http"
	"strings"
)

func (s *Server) HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	payload, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "页面资源不可用", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(payload)
}

func (s *Server) HandleAssets(w http.ResponseWriter, r *http.Request) {
	clone := r.Clone(r.Context())
	clone.URL.Path = strings.TrimPrefix(r.URL.Path, "/assets")
	s.assets.ServeHTTP(w, clone)
}

func (s *Server) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "声境标注放行台"})
}

var verifyTemplate = template.Must(template.New("verify").Parse(`<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>发布凭据核验</title><link rel="stylesheet" href="/assets/app.css"></head>
<body><main class="verify"><p class="eyebrow">只读核验视图</p><h1>研究数据集发布凭据</h1>
<section class="card"><div class="verified">{{if .Verified}}✓ 摘要校验通过{{else}}✕ 摘要校验失败{{end}}</div>
<dl><dt>凭据 ID</dt><dd>{{.Credential.ID}}</dd><dt>数据集 ID</dt><dd>{{.Credential.DatasetID}}</dd>
<dt>发布序号</dt><dd>{{.Credential.Sequence}}</dd><dt>签发人</dt><dd>{{.Credential.IssuedBy}}</dd>
<dt>签发时间</dt><dd>{{.Credential.IssuedAt}}</dd><dt>清单摘要</dt><dd class="digest">{{.Credential.ManifestDigest}}</dd>
<dt>冻结版本</dt><dd>{{.Credential.DatasetVersion}}</dd><dt>片段数</dt><dd>{{len .Manifest.Clips}}</dd></dl></section>
<p><a href="/">返回工作台</a></p></main></body></html>`))

func (s *Server) HandleVerifyPage(w http.ResponseWriter, r *http.Request) {
	view, err := s.service.VerifyCredential(r.PathValue("credentialID"))
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := verifyTemplate.Execute(w, view); err != nil {
		s.logger.Error("渲染核验页失败", "error", err)
	}
}
