# GoUI

GoUI 使用 XML 描述界面、使用 Go 编写业务代码。框架是普通 Go 模块；用户项目自己写 `main.go` 并调用框架 API，不使用代码生成器、独立打包器、反射或热更新。

目前 XML、布局、CPU 软件渲染和桌面窗口后端已连通。桌面窗口支持 Windows/Linux；DRM/KMS 只接受配置并在启动时明确报告尚未实现。框架不把 UI 资源写进 exe，发布时需要将 `ui/` 与编译后的程序一起部署。

## 运行示例

需要 Go 1.25 或更新版本。每个示例都是独立 Go 模块，普通 UI 示例：

```powershell
cd example/hello-world
go run .
```

标准编译同样由 Go 工具链完成：

```powershell
go build -o GoUI.exe .
```

机器人 GIF 表情示例在 `example/robot-face/` 中，同样在该目录执行 `go run .`。设备使用单个 Image 和底部一个透明 Text；浏览器打开 `http://localhost:8080/`，通过表情名称按钮选择动作、循环和打断策略。`GET /api/expressions` 获取可用名称，`POST /api/emotion` 按名称控制，`POST /api/display` 一次接收完整句子，设备自动逐字显示。动作默认完整播放一次后回正常，正常每隔 2～5 秒随机眨眼。具体接口和 Linux 部署见示例 README。

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

`Open` 的参数是直接包含 `config.xml` 和 `main.xml` 的 UI 目录。`Page` 提供 `GetBlock`、`GetText`、`GetImage` 和 `GetButton`，Getter 返回组件和错误，ID 不存在或标签类型不符会立即报错。组件共用 `SetColor`、`SetVisible` 和 `MoveTo`、`RotateTo`、`ScaleTo` 动画方法；Image 还提供 `SetSource`、完整 GIF 的 `Play` 和完成当前 GIF 的 `Finish`，Text 还提供 `SetText`、`SetTextColor`，Button 还提供 `SetDisabled`、`OnMouseEnter`、`OnMouseLeave`、`OnMouseDown`、`OnMouseUp` 和 `OnClick`。Button 的 XML 属性 `hover-color` 可设置鼠标进入时的填充色，移出后自动恢复 `color`；每种鼠标事件在一个按钮上只能绑定一次，XML 不再声明 `on-click`。按钮被隐藏、父节点被隐藏或按钮被禁用时，会取消未完成的按压和悬停；恢复后需要重新按下才能点击。

窗口可以通过 `<window decorated="false">` 隐藏系统标题栏和边框，再将 Button 事件绑定到 `app.MinimizeWindow()`、`app.ToggleMaximizeWindow()` 和 `app.CloseWindow()`。最大化/还原需要 `resizable="true"`。

```go
check(closeButton.OnClick(func(*goui.Page) error {
    return app.CloseWindow()
}))
```

动画方法的时长使用 `time.Duration`，例如 `500*time.Millisecond`。移动目标是父节点坐标中的 `position`，旋转和缩放围绕组件的 `pivot` 执行；Text、Image、Button 因嵌入 Block 也可使用这些方法。动画使用线性插值，时长为 0 时立即生效。`size=-1` 的轴始终按 `margin` 填充，`MoveTo` 只移动固定尺寸的轴。

## 从网络或设备回调更新页面

组件状态和绘制应在窗口主循环中串行修改。初始化和按钮回调直接调用组件方法；HTTP、网络和设备的外部 goroutine 使用 `app.Dispatch`：

```go
err := app.Dispatch(ctx, func(page *goui.Page) error {
    status, err := page.GetText("status")
    if err != nil { return err }
    return status.SetText("收到设备状态")
})
```

`Dispatch` 等待回调执行完成，并返回业务错误或 context 错误；应用未运行或已停止时返回错误。尚未执行就已经取消的任务会被跳过。回调保持简短，不在其中执行网络请求或长时间计算；主循环内不要再次调用 `Dispatch`。

## GIF 表情

表情使用普通的 `<image>`。默认 `playback="auto"`，按 GIF 内的帧延迟和循环次数播放；有限次播放结束后保持末帧。业务代码可用 `Play` 覆盖 GIF 的循环次数，从首帧重播已加载的资源，不必重新读盘：

```go
// 播完整张 GIF 后，在主循环里执行一次回调。
err := face.Play(false, func(*goui.Page) error {
    return face.SetSource("gif/normal.gif")
})
// 持续循环，收到下一条业务指令后再切换。
err = face.Play(true, nil)
```

`Play(false, nil)` 播一次后保持末帧；循环时不能提供完成回调。新一次 Play、SetSource 或隐藏节点会取消旧回调。Play 只接受可见的 auto GIF，回调中直接使用组件，不要再次 Dispatch；回调错误从 Run 返回。机器人项目用此能力实现“动作完成回正常”和随机眨眼。

需要按素材规范控制进入、保持/循环和回正段时，仍可使用 `playback="segmented"` 和 Finish：

```xml
<image id="face" src="gif/smile_[0.5s].gif"
  size="-1,-1" fit="contain" playback="segmented">
  <text id="answer" anchor="0,1" pivot="0,1" size="-1,100"
    margin="40,0,40,0" position="0,-20" wrap="true"
    align="center" valign="bottom" font-size="28" />
</image>
```

| 文件名尾部 | 分段播放规则 |
| --- | --- |
| `_[0.5s]` | 播放前 0.5 秒，停在该位置；调用 `Finish` 后继续播放剩余部分 |
| `_[0.5s,1.5s]` | 0～0.5 秒进入，0.5～1.5 秒循环；请求结束后播放回正段 |
| `_[29]` | 帧索引从 0 开始，播放到第 29 帧并保持；请求结束后继续播放 |
| `_[29,50]` | 29 之前进入，29～50 帧循环（含第 50 帧），之后是回正段 |

第二个数是循环的结束位置，不是循环时长。支持逗号周围的空格及 `2.0 s` 写法；一组参数不能混用秒和帧。时间点不能超出 GIF 总时长，单个保持点可正好等于总时长，此时停在末帧。错误会在加载时报告 XML 文件与行列位置。未启用分段的普通 GIF 不解析这些文件名参数。

Image 和 Block 一样可以组合子节点，先绘制图片，再绘制 Text 等子节点。子节点以 Image 的布局矩形为参考，不随 `contain` 的留白区域变化；`radius` 只裁图片和背景，不裁子树。

使用一个 Image 就可以切换素材。用 `Finish` 请求当前 GIF 完成本轮，在完成回调中换源；普通 auto GIF 从当前进度走到末尾，分段 GIF 则按回正规则完成：

```go
face, err := page.GetImage("face")
if err != nil { return err }
return face.Finish(func(*goui.Page) error {
    return face.SetSource("gif/laugh_[0.5s, 1.0s].gif")
})
```

`Finish` 立即返回，当前这一轮完成后在窗口主循环中回调一次；回调返回错误会结束 `Run`。普通 GIF 不会重新播放开头；分段 GIF 如果仍在进入段，先完成进入，再跳过保持/循环段接入回正；如果已在循环中，播完当前轮再回正。单值保持则从暂停处继续。每次播放只能请求一次 `Finish`，隐藏图片或其父节点会取消待完成回调；重新显示会从首帧播放。已经可见的图片再次 `SetVisible(true)` 不会重播。需要立即切换时直接调用 `SetSource`。

`SetSource` 的路径相对于 `Open` 指定的 UI 目录，沿用当前 Image 的 `fit`、`playback` 和 Block 样式。目标读取、校验成功后才替换；失败保留原图，成功会取消旧回调并从新素材首帧播放，同一路径也会重新加载。子节点及其文字保持不变。只读取当前素材和换源时的目标，不预加载候选列表，不建立全局 GIF 缓存；旧素材不再引用后由 Go 回收。

所有模式都保留 GIF 的原始帧延迟、透明色和局部帧处置规则；文件名只控制分段，不决定速度。播放复用窗口帧循环，不增加 goroutine 或定时器；仅帧变化时更新图片脏区。Image 的缩放、圆角和 Block 动画仍然可用。

`example/robot-face/` 使用 auto GIF 和 Play，不读取文件名中的暂停点。`POST /api/emotion` 接收 `name`、`loop`（默认 false）与 `interrupt`（默认 true）：可立即打断，或完成当前这一轮后切换；等待期间只保留最新请求。单次动作完成后回正常，正常每隔 2～5 秒随机眨眼。接口状态包含实际 `expression`、`loop` 和可选的 `pending`。`GET /api/expressions` 提供全部可用名称。随机等待和表情策略属于机器人项目，框架只提供通用播放控制。浏览器通过 `Application.Snapshot` 预览窗口实际绘制的画面，动作、随机眨眼和字幕使用同一播放结果，存在采样和网络延迟。

## 配置

`config.xml` 必须注册一个名为 `default` 的字体。窗口配置可省略，默认是在主显示器上打开 960×640 窗口：

```xml
<config>
  <app name="My App" version="1.0.0" />
  <window backend="desktop" display="0" width="960" height="640"
    fullscreen="false" decorated="true" resizable="false"
    min-width="640" min-height="480" />
  <fonts>
    <font name="default" src="fonts/default.ttf" />
  </fonts>
</config>
```

`display` 从 0 开始，0 是系统主显示器。`goui.Displays()` 返回桌面显示器列表及其索引、名称和逻辑尺寸；可用 `len(displays)` 查看显示器数量。窗口模式使用 `width` 和 `height`，全屏模式使用窗口后端报告的目标显示器尺寸，单位是设备独立像素。桌面窗口会按显示器缩放比例创建物理像素级的渲染画布，例如 120% 缩放下，逻辑尺寸 2133×1200 对应约 2560×1440 个渲染像素。XML 中的布局尺寸和字号按渲染像素计算，不会再被桌面缩放二次放大。`backend="drm"` 当前会返回 `DRM backend is not implemented yet`。

`decorated` 默认 `true`，设为 `false` 会隐藏系统标题栏和边框；此时应在 UI 中提供自己的窗口按钮。`resizable` 默认 `false`，设为 `true` 后用户可拖动窗口边缘调整大小，也允许最大化/还原。可选的 `min-width`、`min-height` 使用正整数设备独立像素限制窗口模式的最小尺寸；省略时不限制，且最小尺寸不能超过配置的初始窗口尺寸。

字体路径和图片路径相对于 UI 目录；`component src` 相对于引用所在的 XML 文件。应用名用作窗口标题，版本保留为应用元数据，不写入 Windows exe 版本资源。

## 代码目录

- `application.go`：`Open`、`Application.Page` 和 `Application.Run`，连接 XML 加载、软件渲染与窗口后端。
- `snapshot.go`、`snapshot_test.go`：通过主循环复制已绘制的 RGBA 帧，供外部预览或截图使用；不重新推进动画。
- `dispatch.go`：将外部 goroutine 的页面更新排队，交给窗口主循环执行并返回结果。
- `dispatch_test.go`：更新执行、取消、退出和并发提交测试。
- `page.go`：`Page` 组件查找、Block 通用方法、Text 和 Button 的类型化 API。Text、Image、Button 通过嵌入 Block 共享基础方法。
- `image.go`、`image_test.go`：公开的 Image 类型、`SetSource`、`Play`、完整/分段 GIF 完成回调与 API 转发测试。
- `display.go`：公开桌面显示器信息和 `Displays()` 查询函数。
- `go.mod`、`go.sum`：框架模块地址和依赖版本。
- `internal/ui/`：XML 解析、config 校验、字体与图片资源读取、组件展开和节点模型。
- `internal/layout/`：锚点布局、像素取整和子树重绘范围计算；按钮命中统一由渲染层处理。
- `internal/render/software/snapshot.go`、`snapshot_test.go`：截图内存隔离、绘制前/关闭后的错误测试。
- `internal/render/software/render.go`、`render_test.go`：组装字体、节点绘制顺序、RGBA 帧缓存和组件状态更新；验证绘制、布局变化和缓存行为。
- `internal/ui/image.go`：XML 与运行时换源共用的图片读取、解码和分段校验。
- `internal/ui/animated_image.go`：读取 GIF 的帧、延迟、循环次数和帧处置规则，按需合成当前帧。
- `internal/ui/image_segments.go`、`image_segments_test.go`：校验文件名中的秒/帧分段，统一换算为原 GIF 时间轴，检查加载错误。
- `internal/render/software/image_playback.go`：单张 GIF 的时间轴、单次/循环重播、分段保持和回正状态；不依赖窗口。
- `internal/render/software/image_animation.go`：在窗口帧循环中推进可见 GIF、处理完成回调、重置隐藏动画并使图片缓存失效。
- `internal/render/software/image_animation_test.go`、`image_segments_test.go`：普通 GIF 与分段播放、重绘、显隐和回正切换，以及机器人完整播放和字幕透明的回归测试。
- `internal/render/software/animation.go`、`animation_test.go`：移动、旋转、缩放的线性插值，共用仿射变换，以及填充轴与变换命中的回归测试。
- `internal/render/software/input.go`、`input_test.go`：统一处理鼠标事件、按压与悬停状态、逆绘制顺序命中；测试事件回调中显隐、禁用和小数位移边缘。
- `internal/render/software/shapes.go`、`text.go`、`images.go`：圆角矩形、文字排版、图片换源和缩放绘制。
- `internal/render/software/image_control_test.go`：完整 GIF 的单次/持续播放、完成本轮、完成时机、旧回调取消、切换首帧及错误处理测试。
- `internal/render/software/image_source_test.go`：按需加载、换源失败保留旧图、子节点和完成回调换源的测试。
- `internal/backend/backend.go`：帧来源、呈现后端、排队更新和指针输入接口。
- `internal/backend/window/`：Ebitengine 桌面窗口、Windows/Linux 显示器枚举和其它系统的明确错误实现。
- `doc/`：可直接打开的 XML 教程页面、样式和侧栏导航脚本。
- `example/hello-world/`：可直接运行的独立 Go 项目示例。
- `example/hello-world/main.go`：示例业务入口和按钮事件绑定。
- `example/hello-world/ui/`：示例 `config.xml`、`main.xml`、组件、字体和图片资源。
- `example/robot-face/`：独立机器人 GIF 表情示例，1080×720 黑底窗口，单个 Image 按需播放 23 张 GIF，提供按名播放、循环/打断策略、统一文字接口和 Web 调试面板。
- `example/robot-face/main.go`：机器人项目启动入口、HTTP 与窗口生命周期。
- `example/robot-face/internal/robot/`：项目自己的表情名称、切换策略与统一文字显示，不属于 GoUI 框架。
- `example/robot-face/internal/robot/controller.go`、`emotion.go`：绑定 Image/Text、保存实际状态、登记表情名称与资源。
- `example/robot-face/internal/robot/playback.go`：立即或完成本轮后切换、单次回正常、循环与旧通知失效。
- `example/robot-face/internal/robot/effects.go`、`effects_test.go`：随机眨眼与逐字显示共用的可取消等待循环、过期通知检查。
- `example/robot-face/internal/robot/display.go`、`display_test.go`：完整句子输入、打字机进度、新句子替换与清空。
- `example/robot-face/internal/httpapi/`：项目自己的 Gin 接口、输入校验、Web 面板与测试。
- `example/robot-face/internal/httpapi/web/`：无需前端工具链的调试网页，包含实际 UI 帧预览、表情名称按钮、循环/打断选项、完整句子发送、清空与打字进度。
- `example/robot-face/internal/testui/`：仅供示例测试使用的小尺寸资源。
- `example/robot-face/ui/`：表情与底部文字页面、窗口配置和原始素材。

仓库根目录的 `internal/` 属于框架；`example/robot-face/internal/` 属于独立示例项目。框架不依赖 Gin，也不定义表情名称、倾听状态或大模型协议。用户业务只需要导入根包 `github.com/lwy183178053/GoUI`。

## 检查

在框架仓库根目录运行：

```powershell
$env:CGO_ENABLED = '0'
go test ./...
go vet ./...
```

示例模块单独检查：

```powershell
go -C example/hello-world test ./...
go -C example/robot-face test ./...
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
