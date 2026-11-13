# 机器人 GIF 表情示例

设备窗口只显示黑底 GIF。表情由 Gin HTTP 接口控制，浏览器提供调试面板；大模型与调试面板调用同一个接口。

## 启动

在本目录运行：

```sh
go run .
```

默认窗口为 1080×720，无标题栏、不可拉伸。浏览器打开 `http://localhost:8080/`；从其它机器访问时，使用设备的 IP 和 8080 端口。监听地址和 UI 目录可以通过参数调整：

```sh
go run . -listen :8080 -ui ./ui
```

`-listen :8080` 监听所有网卡；只在本机调试可用 `-listen 127.0.0.1:8080`。窗口正常关闭或按 Ctrl+C 后，HTTP 服务一并退出。无标题栏时也可以用 Alt+F4 关闭桌面窗口。

## 二维坐标

输入始终是 `[情绪值, 唤醒度]`，两个值都为 0～10 的整数。

| 维度 | 0 | 5 | 10 |
| --- | --- | --- | --- |
| `valence`，情绪值 | 消极 | 中性 | 积极 |
| `arousal`，唤醒度 | 平静、困倦 | 普通活跃程度 | 激动、兴奋 |

`emotion.go` 的 `expressions` 列表给每张 GIF 登记名称、坐标和文件名。程序计算 `dx*dx + dy*dy`，选择距离最小的一张；等距时采用列表中较早的一项。23 张素材使用不同坐标，每一张都可以通过它自身的坐标直接选中。

这些坐标是示例自己的可调配置，不是 PDF 给出的精确数值。音乐、运动等素材还带有场景含义，这里也先用一个坐标登记，便于统一测试。

- `[5,5]`：正常，程序启动时的状态。
- `[9,9]`：开心。
- `[1,9]`：生气。
- `[1,2]`：难过。
- `[5,0]`：睡觉。

输入改变但仍然选中同一张 GIF 时，继续当前播放进度；选中另一张时，从它的第一帧开始。所有 GIF 都按文件本身的帧延迟和循环次数播放，不根据文件名推算速度，也不解析文件名中的进入段或循环段。

## HTTP 接口

### 设置表情

```http
POST /api/emotion
Content-Type: application/json

{"valence":9,"arousal":9}
```

Linux / macOS 的调用示例：

```sh
curl -X POST http://127.0.0.1:8080/api/emotion \
  -H 'Content-Type: application/json' \
  -d '{"valence":9,"arousal":9}'
```

Windows PowerShell：

```powershell
Invoke-RestMethod -Method Post `
  -Uri 'http://127.0.0.1:8080/api/emotion' `
  -ContentType 'application/json' `
  -Body '{"valence":9,"arousal":9}'
```

大模型工具可以直接把两个整数作为参数，向设备地址发送同样的 JSON；服务端负责选择 GIF。

### 读取状态

```http
GET /api/emotion
```

GET 和成功的 POST 返回同样的结构：

| 字段 | 含义 |
| --- | --- |
| `current.vector.valence` / `arousal` | 最近一次成功应用的输入坐标 |
| `current.expression` | 当前 GIF 的名称、中文名称、登记坐标和文件名 |
| `expressions` | 所有可选 GIF 及它们的坐标 |

HTTP 返回码：

| 状态码 | 含义 |
| --- | --- |
| `200` | 状态读取成功，或坐标已在窗口主循环中应用 |
| `400` | 缺少字段、非整数、越界、未知字段或 JSON 格式不正确 |
| `503` | 窗口尚未启动、已经退出，或页面无法接受更新 |
| `504` | 等待窗口应用更新超过 3 秒 |

失败响应使用 `{"error":"错误说明"}`。非法输入不会修改当前表情。

## Web 调试面板

- 点击矩阵格子发送整数坐标。深色格子标记输入位置，浅蓝格子标记选中的素材。
- 输入两个数后点击“应用向量”，也可以点击下方的表情名称直接发送素材坐标。
- 面板每秒读取设备状态，因此大模型从其它客户端切换表情时，浏览器也会更新。
- 右侧是素材预览，GIF 播放进度与设备独立；设备窗口才是实际显示输出。
- 页面、样式和脚本通过标准库 `embed` 随程序编译，不依赖前端构建工具或外部 CDN。

## Linux 部署

每个示例有自己的 `go.mod`。在仓库内，可以直接从本目录编译：

```sh
CGO_ENABLED=0 go build -o robot-face .
./robot-face -ui ./ui -listen :8080
```

从 Windows 交叉编译到 Linux ARM64：

```powershell
$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
$env:GOARCH = 'arm64'
go build -o robot-face-linux-arm64 .
```

将可执行文件与 `ui/` 一起复制到设备；从包含它们的目录启动，或用 `-ui` 指定资源目录的绝对路径。Web 文件已经编进程序，不用另外复制 `web/`。

```text
部署目录/
  robot-face
  ui/
    config.xml
    main.xml
    fonts/
    gif/
```

窗口配置位于 `ui/config.xml`。设备独占显示器时可以设置 `fullscreen="true"`；全屏采用所选显示器的实际分辨率。桌面调试保留 `fullscreen="false"` 即可。

当前 `desktop` 后端运行时仍需要可用的 Linux 桌面显示环境。DRM/KMS 后端尚未实现；无桌面设备的直接屏幕输出不在这次接口改造中。

## 文件职责

| 文件或目录 | 职责 |
| --- | --- |
| `main.go` | 参数、UI 加载、HTTP 服务启动、窗口主循环和退出收尾 |
| `emotion.go` | 二维坐标、23 张 GIF 的登记表、最近选择和图片切换 |
| `server.go` | Gin 路由、请求校验、状态响应、内嵌网页和 GIF 访问 |
| `main_test.go` | 小型测试资源、页面加载和下载素材完整性检查 |
| `emotion_test.go` | 坐标唯一性、最近选择、重复输入和并发更新测试 |
| `server_test.go` | HTTP 输入、响应、静态文件及窗口不可用测试 |
| `web/index.html` | 调试面板结构 |
| `web/styles.css` | 面板样式和窄屏适配 |
| `web/app.js` | 矩阵、输入控件、接口调用和状态同步 |
| `ui/main.xml` | 只有同位置的普通 Image，每次显示一张 |
| `ui/config.xml` | 1080×720 窗口、字体配置 |
| `ui/gif/` | 23 张原始 GIF 与两份参考 PDF；PDF 不参与运行 |
| `ui/fonts/` | 保留现有默认字体，满足通用 UI 配置要求 |
| `Emote Pack.zip` | 原始素材压缩包，保留存档，不参与运行 |
| `go.mod`、`go.sum` | 示例模块及 Gin、GoUI 的依赖 |

HTTP 回调通过 `app.Dispatch` 把更新交给窗口主循环，避免从网络 goroutine 直接修改渲染状态。同一次切换的隐藏、显示在一次任务中完成，中间不会绘制半完成状态。`robot` 的锁只保护对外读取的状态；渲染器仍按原来的串行方式运行。

增加素材时，在 `emotion.go` 加一项唯一坐标，在 `ui/main.xml` 加一个对应 ID 的 Image，再放入 GIF 文件。原始素材、字体和授权说明应按各自的许可分发。

## 检查

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go vet ./...
```
