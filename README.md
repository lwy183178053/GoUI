# GoUI

GoUI 使用 XML 描述界面、使用 Go 编写业务代码。框架是普通 Go 模块；用户项目自己写 `main.go` 并调用框架 API，不使用代码生成器、独立打包器、反射或热更新。

目前 XML、布局、CPU 软件渲染和桌面窗口后端已连通。桌面窗口支持 Windows/Linux；DRM/KMS 只接受配置并在启动时明确报告尚未实现。框架不把 UI 资源写进 exe，发布时需要将 `ui/` 与编译后的程序一起部署。

## 运行示例

需要 Go 1.25 或更新版本。仓库内示例是一个独立 Go 模块：

```powershell
cd GoUIExampleProject
go run .
```

标准编译同样由 Go 工具链完成：

```powershell
go build -o GoUI.exe .
```

`GoUI.exe` 运行时需要同目录下的 `ui/` 资源目录。示例通过 `goui.Open("./ui")` 加载它，因此从项目目录启动程序。

## 用户项目

```text
MyApp/
  go.mod
  main.go
  ui/
    config.xml
    main.xml
    component/
    assets/
    fonts/
```

项目通过 Go module 引用 GoUI。仓库示例的 `go.mod` 使用 `replace` 指向本地框架，正式项目可使用发布的模块版本。

最小入口示例：

```go
package main

import (
    "log"
    goui "github.com/lwy183178053/GoUI"
)

func main() {
    app, err := goui.Open("./ui")
    if err != nil {
        log.Fatal(err)
    }

    page := app.Page()
    status, err := page.GetText("status")
    if err != nil {
        log.Fatal(err)
    }
    button, err := page.GetButton("run")
    if err != nil {
        log.Fatal(err)
    }
    if err := button.OnClick(func(page *goui.Page) error {
        return status.SetText("按钮已执行")
    }); err != nil {
        log.Fatal(err)
    }

    if err := app.Run(); err != nil {
        log.Fatal(err)
    }
}
```

`Open` 的参数是直接包含 `config.xml` 和 `main.xml` 的 UI 目录。`Page` 提供 `GetBlock`、`GetText`、`GetImage`、`GetButton`，Getter 返回组件和错误，ID 不存在或标签类型不符会立即报错。组件共用 `SetColor`、`SetVisible` 和 `MoveTo`、`RotateTo`、`ScaleTo` 动画方法；Text 还提供 `SetText`、`SetTextColor`，Button 还提供 `SetDisabled`、`OnMouseEnter`、`OnMouseLeave`、`OnMouseDown`、`OnMouseUp` 和 `OnClick`。每种鼠标事件在一个按钮上只能绑定一次，XML 不再声明 `on-click`。

动画方法的时长使用 `time.Duration`，例如 `500*time.Millisecond`。移动目标是父节点坐标中的 `position`，旋转和缩放围绕组件的 `pivot` 执行；Text、Image、Button 因嵌入 Block 也可使用这些方法。动画使用线性插值，时长为 0 时立即生效。

## 配置

`config.xml` 必须注册一个名为 `default` 的字体。窗口配置可省略，默认是在主显示器上打开 960×640 窗口：

```xml
<config>
  <app name="My App" version="1.0.0" />
  <window backend="desktop" display="0" width="960" height="640" fullscreen="false" />
  <fonts>
    <font name="default" src="fonts/default.ttf" />
  </fonts>
</config>
```

`display` 从 0 开始，0 是系统主显示器。`goui.Displays()` 返回桌面显示器列表及其索引、名称和尺寸；可用 `len(displays)` 查看显示器数量。窗口模式使用 `width` 和 `height`，全屏模式使用窗口后端报告的目标显示器尺寸。尺寸单位是设备独立像素，高 DPI 缩放时可能不同于面板物理像素。`backend="drm"` 当前会返回 `DRM backend is not implemented yet`。

字体路径和图片路径相对于 UI 目录；`component src` 相对于引用所在的 XML 文件。应用名用作窗口标题，版本保留为应用元数据，不写入 Windows exe 版本资源。

## 代码目录

- `application.go`：`Open`、`Application.Page` 和 `Application.Run`，连接 XML 加载、软件渲染与窗口后端。
- `page.go`：公开的 `Page` 及 Block、Text、Image、Button 类型化 API。Text、Image、Button 通过 Go 结构体嵌入 Block，共享基础方法。
- `display.go`：公开桌面显示器信息和 `Displays()` 查询函数。
- `go.mod`、`go.sum`：框架模块地址和依赖版本。
- `internal/ui/`：XML 解析、config 校验、字体/图片读取、组件展开和节点模型。
- `internal/layout/`：锚点布局、像素取整、按钮命中和重绘范围计算。
- `internal/render/software/render.go`：组装字体、节点绘制顺序、RGBA 帧缓存和组件状态更新。
- `internal/render/software/animation.go`：移动、旋转、缩放的线性插值，以及绘制和命中共用的仿射变换。
- `internal/render/software/input.go`：鼠标事件注册、按钮事件派发和逆绘制顺序命中。
- `internal/render/software/shapes.go`、`text.go`、`images.go`：圆角矩形、文字排版和图片缩放绘制。
- `internal/backend/backend.go`：帧来源、呈现后端和指针输入接口。
- `internal/backend/window/`：Ebitengine 桌面窗口、Windows/Linux 显示器枚举和其它系统的明确错误实现。
- `doc/`：可直接打开的 XML 教程页面、样式和侧栏导航脚本。
- `GoUIExampleProject/`：拥有独立 `go.mod` 的完整用户项目示例。
- `GoUIExampleProject/main.go`：示例业务入口和按钮事件绑定。
- `GoUIExampleProject/ui/`：示例 `config.xml`、`main.xml`、组件、字体和图片资源。

框架内部实现都留在 `internal/`，用户业务只需要导入根包 `github.com/lwy183178053/GoUI`。

## 检查

在框架仓库根目录运行：

```powershell
$env:CGO_ENABLED = '0'
go test ./...
go vet ./...
```

示例模块单独检查：

```powershell
cd GoUIExampleProject
go test ./...
```

无 CGO 桌面交叉编译检查：

```powershell
$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
go build ./...
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
go build ./...
```

交叉编译只验证代码可构建，不代表已在目标桌面环境或 DRM 硬件上完成实机验证。
