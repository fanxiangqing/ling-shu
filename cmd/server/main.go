package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ling-shu/internal/bootstrap"
	"ling-shu/internal/config"
	logpkg "ling-shu/pkg/log"

	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "config file path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		panic(err)
	}

	logger, err := logpkg.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	app, err := bootstrap.BuildApplication(context.Background(), cfg, logger)
	if err != nil {
		logger.Fatal("build application failed", zap.Error(err))
	}

	server := app.Server()
	startServer(server, logger, cfg.App.Env)
	waitForShutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("server shutdown failed", zap.Error(err))
	}

	app.Close(logger)
	logger.Info("ling-shu server stopped")
}

func startServer(server *http.Server, logger *zap.Logger, env string) {
	go func() {
		logger.Info("ling-shu server started", zap.String("addr", server.Addr), zap.String("env", env))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server stopped unexpectedly", zap.Error(err))
		}
	}()
}

func waitForShutdown() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
}
