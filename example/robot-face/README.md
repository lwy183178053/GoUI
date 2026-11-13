# 机器人表情与文字示例

这是一个独立 Go 项目。屏幕用一个 Image 播放当前 GIF，内部一个底部 Text 显示所有文字。项目通过 Gin 提供表情列表、按名称播放和文字接口，浏览器提供调试面板。

## 启动

在本目录执行：

```sh
go run .
```

浏览器打开 `http://localhost:8080/`。默认窗口 1080×720、无标题栏、不可拉伸；其它设备通过本机 IP 和 8080 端口调试。

```sh
go run . -ui ./ui -listen :8080
```

`-listen :8080` 监听所有网卡；只在本机调试时可使用 `127.0.0.1:8080`。关闭窗口、Alt+F4 或 Ctrl+C 后，HTTP 服务与眨眼等待任务一并退出。

## 目录与代码边界

```text
robot-face/
  go.mod
  main.go                     # 启动入口、HTTP 与窗口生命周期
  internal/                   # 全部是这个机器人项目自己的代码
    robot/
      controller.go           # 绑定 Image/Text，保存实际状态和待切换请求
      emotion.go              # 表情名称、中文显示名和 GIF 文件登记表
      playback.go             # 立即/延后切换、单次/循环、动作结束回正常
      idle.go                 # 正常表情随机眨眼，可取消的定时等待
      display.go              # 唯一文字区域的替换、追加与输入校验
      *_test.go               # 表情、切换时序、眨眼、文字和并发测试
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
        app.js                # 名称按钮、循环/打断选项和状态轮询
        display.js            # 文字编辑、追加与模拟流式输出
        preview.js            # 串行获取真实画面，更新固定 Canvas
    testui/
      fixture.go              # 测试专用的小型 GIF 与字体
  ui/
    config.xml                # 窗口、应用名与字体配置
    main.xml                  # 一个 Image + 一个透明背景的底部 Text
    fonts/
    gif/                      # 原始 GIF 素材
```

仓库根目录的 `image.go`、`page.go` 和 `internal/` 是 **GoUI 框架代码**。本目录的 `internal/robot` 和 `internal/httpapi` 是 **机器人项目业务代码**。框架不依赖 Gin，也不定义表情名称、随机眨眼间隔或大模型协议。

框架负责 `Image.SetSource`、`Image.Play`、`Image.Finish`、GIF 解码、文字绘制、布局、窗口、`Application.Dispatch` 与 `Application.Snapshot`。机器人项目负责选择 GIF、播放策略、动作结束回正常和文字内容。后续语音或设备功能可继续放在本项目的 `internal/` 下。

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

动作播放一次后回到 `normal`。正常 GIF 自带一次眨眼：播完后保持睁眼，随机等待 **2～5 秒**再重播。正常表情忽略 loop，不会连续快速眨眼。间隔在 `internal/robot/idle.go` 的两个常量中调整。

随机等待只有一个可取消的业务循环；每次 GIF 完成后才安排下一次眨眼。新动作清除原眨眼计划，已经排队的旧通知也会在主循环重新检查，避免打断新动作。GIF 帧仍由框架原有帧循环推进。

## 一个 Image 按需换源

`ui/main.xml` 只声明当前表情 Image，不为全部 GIF 建立重叠节点：

```xml
<ui color="#000000">
  <image id="face" size="-1,-1" fit="contain" src="gif/smile_[0.5s].gif">
    <text id="caption" anchor="0,1" pivot="0,1" size="-1,120"
      margin="40,0,40,0" position="0,-24" visible="false"
      font-size="28" text-color="#F2F6FA" align="center" valign="bottom" wrap="true" />
  </image>
</ui>
```

项目只保存文件名登记表。启动只加载初始资源，换表情才调用 `SetSource` 读取目标；同一动作和正常眨眼调用 `Play` 重播，不重新读盘。旧素材的画布和完成回调随换源释放，不缓存全部候选 GIF。换源不会重建或清空文字节点。

文字以 Image 的完整布局矩形定位，contain 留白不会缩放文字。背景完全透明，内容为空时隐藏；所有提示和回答共用 `caption`。窗口主循环中的完成回调直接操作组件，HTTP 与眨眼等待任务通过 `app.Dispatch` 提交更新。

## 文字接口

`GET /api/display` 读取当前文字。`POST /api/display` 只接受以下操作之一：

| JSON | 含义 |
| --- | --- |
| `{"text":"正在倾听…"}` | 替换全部文字 |
| `{"text":"你好"}` | 用第一段回答替换提示 |
| `{"append":"，很高兴见到你。"}` | 追加模型生成片段 |
| `{"text":""}` | 清空并隐藏文字 |

响应为 `{"current":{"text":"当前文字"}}`。text 与 append 不能同时提供；累计最多 8192 个字符，超出时返回 400 并保留原文字。提示和回答会互相替换，所有文字只在同一区域显示。长回答在底部显示最后几行，切换表情不清空内容。

## Web 调试面板

- 页面从 `GET /api/expressions` 创建表情名称按钮，不重建列表。
- 点击前选择“持续循环”和“立即打断”，选项用于下一次点击；默认播一次且立即打断。
- 实际表情以浅蓝标记，待切换表情以虚线标记，状态文字说明当前动作与目标。
- 预览通过 `GET /api/preview` 获取窗口最近绘制的真实帧，包含动作、随机眨眼和文字；浏览器不单独播放 GIF。
- 预览最多约 20 次/秒串行取帧，宽度最多 640px、JPEG 品质 85；存在少量采样和网络延迟。
- 文字控件支持替换、清空、追加和模拟流式回答，轮询不会覆盖尚未提交的文字草稿。

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
