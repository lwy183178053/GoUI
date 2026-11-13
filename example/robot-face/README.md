# 机器人表情与回答示例

这是一个独立 Go 项目。屏幕上的一个 Image 播放当前 GIF，内部的 Text 在底部显示回答和倾听、思考等状态；Gin 提供表情和文字接口，浏览器提供调试面板。

## 启动

在本目录执行：

```sh
go run .
```

浏览器打开 `http://localhost:8080/`。默认窗口 1080×720、无标题栏、不可拉伸；从其它设备调试时使用本机 IP 和 8080 端口。参数示例：

```sh
go run . -ui ./ui -listen :8080
```

`-listen :8080` 监听所有网卡，只在本机调试可使用 `127.0.0.1:8080`。关闭窗口、Alt+F4 或 Ctrl+C 后，HTTP 服务一并退出。

## 目录与代码边界

```text
robot-face/
  go.mod
  main.go                     # 启动、HTTP 和窗口生命周期
  internal/                   # 全部是这个机器人项目自己的代码
    robot/
      controller.go           # 绑定页面组件，提交向量，完成回正后换源
      emotion.go              # 情绪坐标、23 张素材的登记表与最近选择
      display.go              # 回答全文、增量追加、倾听等交互状态
      *_test.go               # 表情、文字和并发更新测试
    httpapi/
      server.go               # Gin 路由、内嵌网页、GIF 白名单访问
      emotion.go              # /api/emotion
      display.go              # /api/display
      request.go              # JSON 限制和错误状态码
      *_test.go               # HTTP 输入输出测试
      web/
        index.html            # 调试页面
        styles.css            # 桌面与手机布局
        app.js                # 表情向量控件和状态轮询
        display.js            # 回答、交互状态、模拟流式输出
    testui/
      fixture.go              # 测试专用的小型 GIF 和字体，不参与程序运行
  ui/
    config.xml                # 窗口与字体配置
    main.xml                  # 一个 Image + 底部文字子节点
    fonts/
    gif/                      # 原始素材；PDF 只作参考，不参与运行
```

仓库根目录的 `page.go`、`image.go` 和 `internal/` 是 **GoUI 框架代码**。这里的 `internal/robot` 和 `internal/httpapi` 是 **项目业务代码**，只有本示例会使用。GoUI 不依赖 Gin，不定义情绪矩阵、倾听状态或大模型接口。

框架负责 `Image.SetSource`、`Text.SetText`、分段播放、布局、绘制、窗口以及 `Application.Dispatch`。项目负责“选择哪个 GIF”“回答显示什么”“何时进入倾听”，HTTP 层将请求交给项目控制器，再通过普通组件 API 更新页面。

后续增加语音、设备控制等功能，可在此项目的 `internal/` 下新增对应功能目录，或在 `robot/` 扩展显示业务；不把业务塞进框架渲染器。

## 一个 Image 按需换源

`ui/main.xml` 只声明一个表情 Image，不再为全部 GIF 建立重叠节点：

```xml
<ui color="#000000">
  <image id="face" size="-1,-1" fit="contain" playback="segmented"
    src="gif/smile_[0.5s].gif">
    <!-- 实际文件中包含底部 caption Block、status 和 answer Text。 -->
    <text id="answer" anchor="0,1" pivot="0,1" size="-1,100"
      position="0,-20" margin="40,0,40,0" font-size="28"
      align="center" valign="bottom" wrap="true" />
  </image>
</ui>
```

Image 继承 Block 的坐标空间，子节点以它的完整布局矩形为参考，先绘制图片再绘制文字。`contain` 的留白不会缩放子节点，圆角也不裁子树。底部回答为空且状态为 `idle` 时，整个字幕区域隐藏。

项目只保存 23 个文件名和坐标。启动只解码初始 `src`；切换时调用 `face.SetSource("gif/目标文件.gif")`，框架才读取目标。读取、校验成功后才替换，旧素材、播放画布和待完成回调不再保留；不会建立全部 GIF 的全局缓存。换源时，原有文字组件不重建、不清空。

`SetSource` 成功后从第一帧播放，同一路径也重新加载。需要自然回正时先调用 `Finish`，在完成回调中换源；需要立即切换时直接 `SetSource`。组件更新放在窗口主循环，HTTP 请求使用 `app.Dispatch` 提交。

## 表情坐标和分段

输入顺序是 `[情绪值, 唤醒度]`，两项均为 0～10 的整数：

| 维度 | 0 | 5 | 10 |
| --- | --- | --- | --- |
| `valence` | 消极 | 中性 | 积极 |
| `arousal` | 平静、困倦 | 普通活跃程度 | 激动、兴奋 |

`internal/robot/emotion.go` 给每张 GIF 登记坐标，计算 `dx*dx + dy*dy` 选择最近的一张；等距时采用登记顺序。坐标是项目自己的可调配置，不是参考 PDF 中的精确数值。常用坐标：正常 `[5,5]`、开心 `[9,9]`、生气 `[1,9]`、难过 `[1,2]`、睡觉 `[5,0]`。

| 文件名参数 | 进入与保持/循环 | 请求结束后 |
| --- | --- | --- |
| `[0.5s]` | 播放到 0.5 秒后保持 | 从保持位置继续到末尾 |
| `[0.5s,1.5s]` | 前 0.5 秒进入，循环 0.5～1.5 秒 | 完成当前轮后播放回正段 |
| `[29]` | 播放到第 29 帧后保持 | 从该位置继续 |
| `[29,50]` | 29 之前进入，循环 29～50 帧，含末帧 | 完成当前轮后回正 |

帧索引从 0 开始，第二个参数是结束位置，不是时长。支持逗号周围空格及 `2.0 s` 写法；原始帧延迟不变。仍在进入段时收到切换请求，会先完成进入再接回正。像睡觉的 21.5 秒进入段较长，过早切换也会等到它完成。

同一张表情重复选中会继续播放。回正期间只保留最新目标，不积压旧目标；目标改回当前表情时，也先完成回正再重新进入。没有新请求时，单值保持、双值循环。普通图片默认 `playback="auto"`，不解析文件名分段。

## 表情接口

```http
POST /api/emotion
Content-Type: application/json

{"valence":9,"arousal":9}
```

`GET /api/emotion` 和成功的 POST 都返回：

| 字段 | 含义 |
| --- | --- |
| `current.vector` | 最新接收的坐标 |
| `current.expression` | 当前实际显示的表情 |
| `current.target` | 最新目标表情 |
| `current.transitioning` | 是否等待当前表情回正 |
| `expressions` | 全部素材的名称、标签、坐标和文件名 |

POST 成功表示目标已提交，不等待整段回正完成；轮询 GET 可确认实际显示状态。大模型控制例子：

```sh
curl -X POST http://127.0.0.1:8080/api/emotion \
  -H 'Content-Type: application/json' \
  -d '{"valence":9,"arousal":9}'
```

## 回答文字和交互状态接口

`POST /api/display` 只修改提供的字段，不会强制改变表情。`GET /api/display` 读取同样的显示状态。

| 字段 | 用法 |
| --- | --- |
| `text` | 替换回答全文；空字符串清空回答 |
| `append` | 将新片段追加到当前回答，供模型增量输出使用 |
| `status` | `idle`、`listening`、`thinking`、`speaking` |

至少提供一项；`text` 和 `append` 不能同时出现。状态可单独修改，省略的字段保留原值。累计回答最多 8192 个 Unicode 字符，POST 请求体最多 64 KiB。

正常调用顺序：

```json
{"text":"","status":"listening"}
```

```json
{"status":"thinking"}
```

```json
{"text":"","status":"speaking"}
```

模型每生成一个片段，就按顺序发送一条请求并等待成功，再发送下一条：

```json
{"append":"你好，"}
```

```json
{"append":"今天天气很好。"}
```

回答结束后隐藏状态文字，保留回答：

```json
{"status":"idle"}
```

成功响应示例：

```json
{"current":{"text":"你好，今天天气很好。","status":"idle"}}
```

这是连续提交片段的 HTTP 接口，项目不接入具体大模型服务，不自动生成文字。并行客户端按窗口主循环实际接收的顺序追加；单次回答应由调用方按生成顺序发送。长回答在底部区域显示末尾几行，以便看到最新输出；完整内容可通过 GET 或 Web 面板读取。新一轮倾听不会隐式清空旧回答，需要同时传 `text:""`。

Windows PowerShell 例子：

```powershell
Invoke-RestMethod -Method Post `
  -Uri 'http://127.0.0.1:8080/api/display' `
  -ContentType 'application/json; charset=utf-8' `
  -Body '{"text":"你好，我在听。","status":"listening"}'
```

两类接口的错误响应均为 `{"error":"错误说明"}`：输入错误返回 `400`，窗口不可用返回 `503`，等待主循环超过 3 秒返回 `504`。非法输入保留原状态。

## Web 调试

- 矩阵、预设和整数输入控制表情，页面区分当前、目标与回正状态。
- 回答区域支持替换全文、追加片段、修改状态和清空。
- “模拟流式回答”将输入文字按 8 个字符一段依次发送，可直接检查屏幕的实时更新。
- 面板每秒读取设备状态；原始 GIF 预览不模拟分段、回正或底部字幕，实际播放以设备窗口为准。
- 网页编进程序，不使用外部 CDN 或前端构建工具。

## 构建与部署

在本目录编译：

```sh
CGO_ENABLED=0 go build -o robot-face .
./robot-face -ui ./ui -listen :8080
```

Windows 交叉编译 Linux ARM64：

```powershell
$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
$env:GOARCH = 'arm64'
go build -o robot-face-linux-arm64 .
```

部署时保留可执行文件和 `ui/`，网页已内嵌，无需另外复制 `internal/httpapi/web/`。修改 `ui/config.xml` 可调整窗口与全屏；`fullscreen="true"` 时采用目标显示器分辨率。

当前 `desktop` 后端需要可用的 Linux 桌面环境。DRM/KMS 尚未实现；交叉编译不代表完成无桌面设备的屏幕验证。字体和 GIF 按其各自许可分发。

增加表情时，只需放入带合法分段参数的 GIF，并在 `internal/robot/emotion.go` 增加唯一坐标；不需要修改 XML 或增加 Image。

## 检查

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go vet ./...
```
