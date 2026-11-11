# GoUI

GoUI 是一个用 XML 描述界面、用 Go 加载和绘制的 UI 实验项目。当前预览程序在 Windows 第 2 块屏幕全屏显示；XML、布局、图片和文字最终绘制成平台无关的 `image.RGBA`。运行时不使用 CGO。

## 运行

需要 Windows 和 Go 1.25 或更新版本。默认加载 `./GoUI`，也可以指定其它包含 `main.xml`、`config.xml` 的目录：

```powershell
$env:CGO_ENABLED = '0'
go run .
go run . -ui .\GoUI
```

预览需要至少两块屏幕，会在系统报告的第 2 块屏幕全屏启动。按 `Alt+F4` 退出。Windows 显示缩放可能使窗口逻辑渲染尺寸小于面板物理分辨率，这是 Ebitengine 使用设备无关像素的结果，不影响窗口铺满屏幕。

## 当前支持

- 标签：`ui`、`block`、`text`、`image`、`button` 和可嵌套的 `component`。
- 布局：`anchor`、`pivot`、`position`、`size`、`margin`；固定尺寸和 `-1` 填充可分别设置在宽、高两个方向。
- 绘制：透明颜色叠加、圆角、字体、文字对齐与换行；图片支持 JPEG、PNG、GIF 静态帧和 `stretch`、`contain`、`cover`。
- 交互：按钮矩形命中、遮挡顺序、禁用状态和按下/释放配对。Go 代码通过 `Renderer.RegisterAction` 注册 XML 的 `on-click` 命令。
- 更新：通过 `SetText`、`SetColor`、`SetTextColor`、`SetVisible`、`SetDisabled` 修改有 ID 的节点；只重绘受影响矩形，并返回新帧供窗口后端更新。
- 组件：加载时透明展开，不产生额外布局层；组件名与节点 ID 自动添加点号前缀，嵌套组件可复用。

DRM/KMS 后端、键盘输入、动画、热加载和自动布局尚未实现。当前 `image.RGBA` 与 `backend.Presenter` 是后端边界；后续 DRM 后端可以复用 XML、布局和软件渲染代码。

## UI 目录

```text
GoUI/
  config.xml                  字体注册表
  main.xml                    主界面
  component/
    display-card.xml          可复用卡片组件
    parts/connection.xml      卡片中的嵌套状态组件
  fonts/                      UI 使用的字体文件
  assets/                     图片资源
```

`config.xml` 必须注册 `name="default"` 的字体。字体路径相对 UI 目录；图片路径也相对 UI 目录；组件 `src` 路径相对当前 XML 文件。组件实例名不能包含点号，且展开后必须全局唯一。组件中的节点 ID 会变成 `组件名.id`，嵌套组件会继续添加名称，例如 `left.status.label`。UI 资源随程序目录一起提供，分发字体时须保留字体授权说明。

## 代码目录与文件

### 根目录

- `main.go`：命令行入口，解析 `-ui`，加载界面并创建渲染器；在这里注册示例按钮的 Go 命令。
- `go.mod`、`go.sum`：Go 版本与依赖版本锁定。
- `README.md`：项目运行方式、当前能力和代码结构。

### `internal/ui`

- `model.go`：平台无关的 UI 节点、通用布局样式、Text/Image/Button 数据和文档对象。
- `xml.go`：用标准库读取 XML token，保留文件、行、列，检查 XML 结构与未知属性。
- `values.go`：解析颜色、整数坐标、0～1 比例、小数和布尔值。
- `load.go`：加载 `config.xml`、字体、`main.xml`、图片，校验标签属性并生成节点树。
- `components.go`：加载并展开组件 XML，检查循环引用和重复组件名，给组件 ID 加命名空间。
- `load_test.go`：配置、XML 错误位置、布局属性、图片与组件展开测试。

### `internal/layout`

- `layout.go`：计算完整节点树的浮点坐标和整数像素矩形；也提供按钮命中和子树范围计算。
- `layout_test.go`：验证锚点、轴心、半像素取整、填充边距、窗口尺寸变化和按钮命中顺序。

### `internal/render/software`

- `render.go`：管理字体、布局缓存和 RGBA 帧；处理按钮命令及节点动态更新，并局部重绘脏区。
- `shapes.go`：绘制填充色和抗锯齿圆角矩形。
- `text.go`：按字体度量绘制文字，处理水平/垂直对齐、换行和文字区域裁切。
- `images.go`：按 stretch、contain、cover 缩放图片，并按 Image 的矩形与圆角裁切。
- `render_test.go`：覆盖背景混色、节点绘制顺序、文字、图片、圆角、按钮和局部刷新。

### `internal/backend`

- `backend.go`：定义 RGBA 帧来源、窗口 Presenter 和鼠标指针输入接口。
- `window/window.go`：设置窗口后端的标题等公共选项。
- `window/window_windows.go`：Windows Ebitengine 窗口；选择第 2 块屏幕、全屏显示帧并转发鼠标点击。
- `window/window_other.go`：非 Windows 系统的占位实现，给出明确的“不支持窗口预览”错误。

### `doc` 与 `GoUI`

- `doc/index.html`：按通用属性、标签、组件和补充说明组织的 XML 使用教程。
- `doc/styles.css`、`doc/docs.js`：教程样式、折叠侧栏、当前章节提示和代码复制按钮。
- `GoUI/main.xml`、`GoUI/config.xml`、`GoUI/component/`：可运行的界面示例、字体配置和组件。
- `GoUI/assets/`、`GoUI/fonts/`：示例图片与字体资源。
- `AlibabaPuHuiTi-3/`：下载的字体原始文件目录；运行时实际使用的是 `GoUI/fonts/` 下配置的文件。

## 测试与构建

```powershell
$env:CGO_ENABLED = '0'
go test ./...
go vet ./...
go build -o .cache/goui.exe .

# 在独立 PowerShell 会话中做 Linux/ARM64 无 CGO 编译检查
$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
$env:GOARCH = 'arm64'
go build -o .cache/goui-linux-arm64 .
```

Linux/ARM64 构建只验证平台无关代码可编译，不表示已经实现或验证 DRM/KMS 实机输出。
