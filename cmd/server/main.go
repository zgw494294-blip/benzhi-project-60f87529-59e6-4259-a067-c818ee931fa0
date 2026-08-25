package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/application"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/httpui"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/repository"
)

type config struct {
	address   string
	dataDir   string
	selfcheck bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "启动失败:", err)
		os.Exit(1)
	}
}

func run() error {
	var cfg config
	flag.StringVar(&cfg.address, "addr", addressDefault(), "HTTP 监听地址")
	flag.StringVar(&cfg.dataDir, "data", "./data", "本地事件与投影目录")
	flag.BoolVar(&cfg.selfcheck, "selfcheck", false, "启动真实监听并执行端到端自检")
	flag.Parse()
	if err := validateAddress(cfg.address); err != nil {
		return err
	}
	if cfg.selfcheck {
		return runSelfcheck(cfg.address)
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	store, err := repository.Open(cfg.dataDir)
	if err != nil {
		return err
	}
	handler := httpui.New(application.NewService(store), logger)
	listener, err := net.Listen("tcp", cfg.address)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.address, err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	logger.Info("声境标注放行台已启动", "address", listener.Addr().String(), "data", cfg.dataDir)
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serveErrors:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	case <-signalContext.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return server.Shutdown(shutdownContext)
	}
}
