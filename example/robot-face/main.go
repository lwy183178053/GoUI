package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	goui "github.com/lwy183178053/GoUI"
)

func main() {
	uiDirectory := flag.String("ui", "./ui", "包含 config.xml 和 main.xml 的 UI 目录")
	listen := flag.String("listen", ":8080", "Web 调试面板和表情接口的监听地址")
	flag.Parse()
	if err := run(*uiDirectory, *listen); err != nil {
		log.Fatal(err)
	}
}

func run(uiDirectory, address string) error {
	directory, err := filepath.Abs(uiDirectory)
	if err != nil {
		return err
	}
	app, err := goui.Open(directory)
	if err != nil {
		return err
	}
	robot, err := newRobot(app.Page(), app.Dispatch)
	if err != nil {
		return err
	}
	// 先占用端口，地址错误或端口冲突时不启动一个无法控制的窗口。
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("启动表情接口: %w", err)
	}
	defer listener.Close()

	gin.SetMode(gin.ReleaseMode)
	server := &http.Server{
		Handler:           newRouter(robot, directory),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serverErrors <- err
		if err != nil {
			requestClose(app)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		requestClose(app)
	}()
	log.Printf("Web 调试面板：http://localhost:%d；远程使用本机 IP 和同一端口", listener.Addr().(*net.TCPAddr).Port)
	log.Print("POST /api/emotion  请求示例：{\"valence\":8,\"arousal\":9}")

	// 窗口循环保留在主 goroutine；HTTP 通过 Dispatch 提交组件更新。
	windowErr := app.Run()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	return errors.Join(windowErr, shutdownErr, <-serverErrors)
}

func requestClose(app *goui.Application) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = app.Dispatch(ctx, func(*goui.Page) error { return app.CloseWindow() })
}
