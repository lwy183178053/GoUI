# 机器人表情、文字与麦克风示例

这是一个独立 Go 项目。屏幕用一个 Image 播放当前 GIF，内部一个底部 Text 显示所有文字，右下角的麦克风按钮用于本地录音。项目通过 Gin 提供表情列表、按名称播放和文字接口，浏览器提供调试面板。

## 启动

在本目录执行：

```sh
go run .
```

浏览器打开 `http://localhost:8080/`。默认窗口 1080×720、无标题栏、不可拉伸；其它设备通过本机 IP 和 8080 端口调试。

```sh
go run . -ui ./ui -listen :8080
```

`-listen :8080` 监听所有网卡；只在本机调试时可使用 `127.0.0.1:8080`。关闭窗口、Alt+F4 或 Ctrl+C 后，HTTP 服务、显示效果等待任务与录音任务一并退出，并释放麦克风。

## 目录与代码边界

```text
robot-face/
  go.mod
  main.go                     # 启动入口、HTTP 与窗口生命周期
  internal/                   # 全部是这个机器人项目自己的代码
    robot/
      controller.go           # 绑定 Image/Text/Button，保存实际状态和待切换请求
      emotion.go              # 表情名称、中文显示名和 GIF 文件登记表
      playback.go             # 立即/延后切换、单次/循环、动作结束回正常
      effects.go              # 眨眼与打字机共用的可取消等待循环
      display.go              # 完整句子输入、逐字显示、清空与校验
      microphone.go           # 麦克风按钮状态、后台录音与结束保存
      *_test.go               # 表情、切换时序、眨眼、文字和并发测试
    voice/
      record.go               # 默认麦克风录音、PCM 读取与统一的一分钟上限
      wav.go                  # WAV 编码与最近一次录音的缓存保存
      input_windows.go        # Windows 默认麦克风，使用系统 WinMM
      input_linux.go          # Linux 默认麦克风，调用 ALSA arecord
      input_other.go          # 其它平台的明确错误提示
      *_test.go               # 录音取消、长度限制、设备错误、WAV 与缓存覆盖测试
    httpapi/
      server.go               # Gin 路由与内嵌网页
      emotion.go              # 表情列表、当前状态和按名播放接口
      display.go              # /api/display 文字接口
      preview.go              # 将真实 UI 帧缩小为预览图
      request.go              # JSON 限制与错误状态码
      *_test.go               # HTTP 输入输出测试
      web/
        index.html            # 调试页面
        styles.css            # 桌面和手机布局
        api.js                # 文字与表情共用的 JSON 请求与错误处理
        app.js                # 名称按钮、循环/打断选项和状态轮询
        display.js            # 完整句子发送、清空与显示进度
        preview.js            # 串行获取真实画面，更新固定 Canvas
    testui/
      fixture.go              # 测试专用的小型 GIF 与字体
  ui/
    config.xml                # 窗口、应用名与字体配置
    main.xml                  # 表情 Image、透明底部 Text 和右下角麦克风 Button
    fonts/
    gif/                      # 原始 GIF 素材
```

仓库根目录的 `image.go`、`page.go` 和 `internal/` 是 **GoUI 框架代码**。本目录的 `internal/robot`、`internal/voice` 和 `internal/httpapi` 是 **机器人项目业务代码**。框架不依赖 Gin，也不定义表情名称、随机眨眼间隔或大模型协议。

框架负责 `Image.SetSource`、`Image.Play`、`Image.Finish`、GIF 解码、文字绘制、布局、窗口、`Application.Dispatch` 与 `Application.Snapshot`。机器人项目负责选择 GIF、播放策略、动作结束回正常、随机眨眼和打字机文字效果。麦克风录音同样属于本项目，放在 `internal/` 下。

## 表情接口

先获取可用名称：

```http
GET /api/expressions
```

```json
{"expressions":[{"name":"normal","label":"正常","file":"smile_[0.5s].gif"}]}
```

上面只列一项。实际返回全部 23 个表情，`name` 用于调用，`label` 用于界面展示。名称区分大小写，以列表为准；不再接收情绪坐标。

播放表情：

```http
POST /api/emotion
Content-Type: application/json

{"name":"music","loop":true,"interrupt":true}
```

| 字段 | 默认值 | 含义 |
| --- | --- | --- |
| `name` | 必填 | 表情列表中的名称，例如 normal、happy、music、badminton |
| `loop` | false | false 完整播放一次后回正常；true 持续循环 |
| `interrupt` | true | true 立即打断当前动作；false 完成当前这一轮后切换 |

例子：

```sh
# 开心一次，完成后自动回正常。
curl -X POST http://127.0.0.1:8080/api/emotion -H 'Content-Type: application/json' -d '{"name":"happy"}'

# 等当前这一轮结束，再开始持续循环音乐。
curl -X POST http://127.0.0.1:8080/api/emotion -H 'Content-Type: application/json' -d '{"name":"music","loop":true,"interrupt":false}'

# 立即回正常，恢复随机眨眼。
curl -X POST http://127.0.0.1:8080/api/emotion -H 'Content-Type: application/json' -d '{"name":"normal"}'
```

`GET /api/emotion` 与成功的 POST 返回当前状态：

```json
{
  "current": {
    "expression": {"name":"music","label":"音乐","file":"music_[2.5s].gif"},
    "loop": false,
    "pending": {"name":"happy","loop":false,"interrupt":false}
  }
}
```

`expression` 是正在显示的表情，`loop` 是它目前是否持续循环。存在 `pending` 表示本轮结束后还有一次切换，没有待切换请求时省略该字段。上例中音乐只再完成这一轮，然后切到开心。

等待期间只保留最新请求，不建立动作队列；新的 `interrupt=true` 请求会取消等待并立即播放。重复请求同一动作也会按所选策略从头重播。正常表情正在等待下次眨眼时，没有动作需要等待，可以直接开始新动作。

未知名称、旧坐标字段、错误的字段类型等返回 HTTP 400；窗口不可用返回 503，调度超时返回 504。

## 正常眨眼与完整播放

所有动作按原 GIF 的帧延迟完整播放。文件名中的 `_[0.5s]`、`_[0.5s,1.5s]` 等仅保留为素材名称，本示例不读取它们作为暂停点，音乐和运动都能完整循环。

动作播放一次后回到 `normal`。正常 GIF 自带一次眨眼：播完后保持睁眼，随机等待 **2～5 秒**再重播。正常表情忽略 loop，不会连续快速眨眼。间隔在 `internal/robot/effects.go` 的两个常量中调整。

随机眨眼与逐字显示共用一个可取消的业务等待循环；每次 GIF 完成后才安排下一次眨眼。新动作清除原眨眼计划，已经排队的旧通知也会在主循环重新检查，避免打断新动作。GIF 帧仍由框架原有帧循环推进。

## 一个 Image 按需换源

`ui/main.xml` 用一个 Image 播放当前表情，不为全部 GIF 建立重叠节点。下面只列出表情与文字部分；完整布局中的麦克风 Button 放在根节点末尾，显示在右下角：

```xml
<ui color="#000000">
  <image id="face" size="-1,-1" fit="contain" src="gif/smile_[0.5s].gif">
    <text id="caption" anchor="0,1" pivot="0,1" size="-1,120"
      margin="112,0,112,0" position="0,-24" visible="false"
      font-size="28" text-color="#F2F6FA" align="center" valign="bottom" wrap="true" />
  </image>
</ui>
```

项目只保存文件名登记表。启动只加载初始资源，换表情才调用 `SetSource` 读取目标；同一动作和正常眨眼调用 `Play` 重播，不重新读盘。旧素材的画布和完成回调随换源释放，不缓存全部候选 GIF。换源不会重建或清空文字节点。

文字以 Image 的完整布局矩形定位，contain 留白不会缩放文字。背景完全透明，内容为空时隐藏；所有提示和回答共用 `caption`。窗口主循环中的完成回调直接操作组件，HTTP 与显示效果等待任务通过 `app.Dispatch` 提交更新。

## 麦克风按钮

右下角始终显示一个 64×64 的圆形麦克风按钮，距窗口右边和下边各 24 像素。图标由普通 Block 组合，不依赖额外图片或图标字体；字幕左右各留出 112 像素，避免与按钮重叠。

- 点击一次：打开默认麦克风。收到第一段音频后按钮亮蓝，底部立即显示“正在听”。
- 再点击一次：停止录音，释放设备并保存最近一次 WAV。单次录音最多一分钟，到时自动结束。
- 设备打开和录音结束期间短暂禁用按钮，录音读取、文件保存均在后台进行，窗口动画继续播放。
- 打开设备失败时，在同一个底部文字区域显示原因；修复设备或权限后可再次点击。
- 关闭窗口时取消未完成的录音并释放设备，退出时不保存半途取消的片段。

音频为 **16kHz、单声道、16 位小端 PCM 的 WAV**。只保留最近一次录音，路径打印在终端中：

- Windows：`%LOCALAPPDATA%/GoUI/robot-face/last-recording.wav`。
- Linux：`$XDG_CACHE_HOME/GoUI/robot-face/last-recording.wav`，未设置缓存目录时使用 `~/.cache/GoUI/robot-face/last-recording.wav`。

Windows 通过系统 WinMM 录音，需要允许桌面应用使用默认麦克风。Linux 使用 `alsa-utils` 中的 `arecord`，程序需要有默认音频设备的访问权限；可先用 `arecord -l` 检查设备。两种方式都可用 `CGO_ENABLED=0` 编译。

本次实现的是按钮和真实本地录音。**VoiceStudio、识别、模型回答和语音输出尚未接入**；当前录音结束后保存文件，并明确显示服务尚未配置。接入语音服务时，在这条录音入口先停止当前回复及待播音频，再开始新的识别。当前开始录音会立即停止旧文字的打字机效果，状态提示不逐字播放。

## 文字接口

一次发送完整句子，设备自动以打字机效果显示：

```http
POST /api/display
Content-Type: application/json

{"text":"你好，很高兴见到你。"}
```

首字立即显示，之后每 **40 毫秒**显示一个字符。中文和 emoji 按 Unicode 字符处理，不会拆开 UTF-8 字节。新请求替换旧句子，从首字重新开始；无需等待上一句播放完。发送 `{"text":""}` 立即清空并隐藏文字。

接口收到后立即返回，不等待整句话显示完成；`GET /api/display` 可读取进度：

```json
{
  "current": {
    "text": "你好，很高兴见到你。",
    "visible_text": "你好",
    "typing": true
  }
}
```

- `text`：最近一次收到的完整句子。
- `visible_text`：当前已经显示的部分。
- `typing`：是否还有字符等待显示，全部显示或清空后为 false。

`text` 是唯一输入字段且必须提供，单次最多 8192 个字符。缺少字段、类型错误、超出限制或未知字段返回 400，原文字与播放进度保留。显示速度在 `internal/robot/display.go` 的 `textCharacterInterval` 常量中调整。

提示和回答共用一个透明背景的文字区域；长回答在底部显示最后几行。切换表情不会清空文字或重置打字进度。HTTP 只提交一次完整句子，后续文字由机器人项目的 `RunEffects` 调度，不为每句话创建 goroutine，框架仍只负责普通 Text 绘制。

## Web 调试面板

- 页面从 `GET /api/expressions` 创建表情名称按钮，不重建列表。
- 点击前选择“持续循环”和“立即打断”，选项用于下一次点击；默认播一次且立即打断。
- 实际表情以浅蓝标记，待切换表情以虚线标记，状态文字说明当前动作与目标。
- 预览通过 `GET /api/preview` 获取窗口最近绘制的真实帧，包含动作、随机眨眼和文字；浏览器不单独播放 GIF。
- 预览最多约 20 次/秒串行取帧，宽度最多 640px、JPEG 品质 85；存在少量采样和网络延迟。
- 文字控件只保留完整句子输入、“发送文字”和“清空”。面板显示打字进度，预览采样设备实际文字；轮询不会用半截结果覆盖完整句子或尚未提交的草稿。

## Linux 部署

Windows 下无 CGO 交叉编译：

```powershell
$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
$env:GOARCH = 'arm64'  # x86_64 机器使用 amd64
go build -o robot-face .
```

Linux 上从包含 ui/ 的目录启动：

```sh
./robot-face -ui ./ui -listen :8080
```

部署时保留可执行文件和 `ui/`；调试网页已内嵌，无需复制 Go 代码或 web/。当前使用 Linux 桌面窗口后端，需要可用的桌面环境；DRM/KMS 仍未实现。`ui/config.xml` 可以设置窗口大小、显示器和全屏，开启全屏时使用所选显示器的分辨率。

增加表情时，放入 GIF 并在 `internal/robot/emotion.go` 登记唯一 name，无需改 XML。素材授权与字体授权随发布保留。

## 检查

```powershell
$env:CGO_ENABLED = '0'
go test ./...
go vet ./...
```
