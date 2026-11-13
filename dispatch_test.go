package goui

import (
	"context"
	"errors"
	"testing"
	"time"
)

// 测试只手动推进主循环，不打开真实窗口。
func dispatchTestApplication(t *testing.T) (*Application, *applicationSource) {
	t.Helper()
	app := &Application{page: &Page{}, updates: make(chan pageUpdate, 64), done: make(chan struct{})}
	app.running.Store(true)
	t.Cleanup(func() {
		if app.running.Swap(false) {
			close(app.done)
		}
	})
	return app, &applicationSource{page: app.page, updates: app.updates}
}

func takeUpdate(t *testing.T, app *Application) pageUpdate {
	t.Helper()
	select {
	case request := <-app.updates:
		return request
	case <-time.After(2 * time.Second):
		t.Fatal("没有收到排队的页面更新")
		return pageUpdate{}
	}
}

func waitDispatch(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("Dispatch 没有及时返回")
		return nil
	}
}

func TestDispatchRunsOnMainLoopAndReturnsCallbackError(t *testing.T) {
	app, source := dispatchTestApplication(t)
	called := false
	want := errors.New("业务更新失败")
	result := make(chan error, 1)
	go func() {
		result <- app.Dispatch(context.Background(), func(page *Page) error {
			if page != app.page {
				return errors.New("页面对象不一致")
			}
			called = true
			return want
		})
	}()
	request := takeUpdate(t, app)
	if called {
		t.Fatal("更新不应在请求 goroutine 内直接执行")
	}
	app.updates <- request
	if err := source.Update(); err != nil {
		t.Fatal(err)
	}
	if err := waitDispatch(t, result); !errors.Is(err, want) || !called {
		t.Fatalf("Dispatch = %v，called = %v", err, called)
	}
}

func TestDispatchSkipsCanceledQueuedUpdate(t *testing.T) {
	app, source := dispatchTestApplication(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	called := false
	result := make(chan error, 1)
	go func() {
		result <- app.Dispatch(ctx, func(*Page) error { called = true; return nil })
	}()
	request := takeUpdate(t, app)
	cancel()
	if err := waitDispatch(t, result); !errors.Is(err, context.Canceled) {
		t.Fatalf("Dispatch = %v，期望 context.Canceled", err)
	}
	app.updates <- request
	if err := source.Update(); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("取消后的排队任务仍然执行了")
	}
}

func TestDispatchUnblocksWhenApplicationStops(t *testing.T) {
	app, _ := dispatchTestApplication(t)
	result := make(chan error, 1)
	go func() {
		result <- app.Dispatch(context.Background(), func(*Page) error { return nil })
	}()
	_ = takeUpdate(t, app)
	app.running.Store(false)
	close(app.done)
	if err := waitDispatch(t, result); err == nil {
		t.Fatal("应用结束后等待中的 Dispatch 没有返回错误")
	}
	if err := app.Dispatch(context.Background(), func(*Page) error { return nil }); err == nil {
		t.Fatal("已关闭的应用仍然接受更新")
	}
}

func TestDispatchProcessesConcurrentRequestsSerially(t *testing.T) {
	app, source := dispatchTestApplication(t)
	const count = 24
	results := make(chan error, count)
	applied := 0
	for range count {
		go func() {
			results <- app.Dispatch(context.Background(), func(*Page) error {
				applied++
				return nil
			})
		}()
	}
	requests := make([]pageUpdate, 0, count)
	for range count {
		requests = append(requests, takeUpdate(t, app))
	}
	for _, request := range requests {
		app.updates <- request
	}
	if err := source.Update(); err != nil {
		t.Fatal(err)
	}
	for range count {
		if err := waitDispatch(t, results); err != nil {
			t.Fatal(err)
		}
	}
	if applied != count {
		t.Fatalf("执行 %d 次，期望 %d 次", applied, count)
	}
}

func TestDispatchValidatesInputsAndRunningState(t *testing.T) {
	callback := func(*Page) error { return nil }
	var app *Application
	if err := app.Dispatch(context.Background(), callback); err == nil {
		t.Fatal("nil 应用没有报错")
	}
	app = &Application{}
	if err := app.Dispatch(context.Background(), callback); err == nil {
		t.Fatal("未启动的应用没有报错")
	}
	if err := app.Dispatch(nil, callback); err == nil {
		t.Fatal("nil context 没有报错")
	}
	if err := app.Dispatch(context.Background(), nil); err == nil {
		t.Fatal("nil 回调没有报错")
	}
}
