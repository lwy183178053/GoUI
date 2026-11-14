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

	"example.com/goui-robot-face/internal/conversation"
	"example.com/goui-robot-face/internal/httpapi"
	"example.com/goui-robot-face/internal/robot"
	"github.com/gin-gonic/gin"
	goui "github.com/lwy183178053/GoUI"
)

func main() {
	uiDirectory := flag.String("ui", "./ui", "包含 config.xml 和 main.xml 的 UI 目录")
	listen := flag.String("listen", ":8080", "Web 调试面板和表情接口的监听地址")
	voiceConfig := flag.String("voice", "./voice.local.json", "语音服务和大模型的本地配置文件")
	flag.Parse()
	if err := run(*uiDirectory, *listen, *voiceConfig); err != nil {
		log.Fatal(err)
	}
}

func run(uiDirectory, address, voiceConfig string) error {
	config, err := conversation.LoadConfig(voiceConfig)
	if err != nil {
		return err
	}
	session := conversation.NewSession(conversation.NewClient(config))
	directory, err := filepath.Abs(uiDirectory)
	if err != nil {
		return err
	}
	app, err := goui.Open(directory)
	if err != nil {
		return err
	}
	controller, err := robot.New(app.Page(), app.Dispatch)
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
		Handler:           httpapi.New(controller, app.Snapshot),
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
	effectsErrors := make(chan error, 1)
	go func() {
		err := controller.RunEffects(ctx)
		effectsErrors <- err
		if err != nil {
			requestClose(app)
		}
	}()
	microphoneErrors := make(chan error, 1)
	go func() {
		err := controller.RunMicrophone(ctx, session)
		microphoneErrors <- err
		if err != nil {
			requestClose(app)
		}
	}()
	go func() {
		<-ctx.Done()
		requestClose(app)
	}()
	log.Printf("Web 调试面板：http://localhost:%d；远程使用本机 IP 和同一端口", listener.Addr().(*net.TCPAddr).Port)
	log.Print("右下角麦克风：点击开始说话，再次点击结束并回答；回答中点击可打断并重新倾听")
	if err := config.Validate(); err != nil {
		log.Printf("语音配置待完成：%v", err)
	}
	log.Print("POST /api/display 传入完整 text，设备自动逐字显示；空字符串清空")
	log.Print("GET /api/expressions 获取列表；POST /api/emotion 示例：{\"name\":\"music\",\"loop\":true,\"interrupt\":true}")

	// 窗口循环保留在主 goroutine；HTTP 通过 Dispatch 提交组件更新。
	windowErr := app.Run()
	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	return errors.Join(windowErr, shutdownErr, <-serverErrors, <-effectsErrors, <-microphoneErrors)
}

func requestClose(app *goui.Application) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = app.Dispatch(ctx, func(*goui.Page) error { return app.CloseWindow() })
}
