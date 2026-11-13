# 机器人表情与文字示例

这是一个独立 Go 项目。屏幕上的一个 Image 播放当前 GIF，内部只有一个 Text，在底部显示所有文字。回答、“正在倾听…”、“正在思考…”等提示都直接写入同一处；Gin 提供表情和文字接口，浏览器提供调试面板。

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
      display.go              # 唯一文字区域的内容替换与增量追加
      *_test.go               # 表情、文字和并发更新测试
    httpapi/
      server.go               # Gin 路由与内嵌网页
      emotion.go              # /api/emotion
      display.go              # /api/display
      preview.go              # 捕获真实 UI 帧，缩小为预览图
      request.go              # JSON 限制和错误状态码
      *_test.go               # HTTP 输入输出测试
      web/
        index.html            # 调试页面
        styles.css            # 桌面与手机布局
        app.js                # 表情向量控件和状态轮询
        display.js            # 文字编辑、增量追加、模拟流式输出
        preview.js            # 串行获取屏幕画面，更新固定 Canvas
    testui/
      fixture.go              # 测试专用的小型 GIF 和字体，不参与程序运行
  ui/
    config.xml                # 窗口与字体配置
    main.xml                  # 一个 Image + 一个底部 Text
    fonts/
    gif/                      # 原始素材；PDF 只作参考，不参与运行
```

仓库根目录的 `page.go`、`image.go` 和 `internal/` 是 **GoUI 框架代码**。这里的 `internal/robot` 和 `internal/httpapi` 是 **项目业务代码**，只有本示例会使用。GoUI 不依赖 Gin，不定义情绪矩阵、倾听状态或大模型接口。

框架负责 `Image.SetSource`、`Text.SetText`、分段播放、布局、绘制、窗口、`Application.Dispatch` 和 `Application.Snapshot`。项目负责“选择哪个 GIF”“文字显示什么”，HTTP 层将请求交给项目控制器，再通过普通组件 API 更新页面。

后续增加语音、设备控制等功能，可在此项目的 `internal/` 下新增对应功能目录，或在 `robot/` 扩展显示业务；不把业务塞进框架渲染器。

## 一个 Image 按需换源

`ui/main.xml` 只声明一个表情 Image，不再为全部 GIF 建立重叠节点：

```xml
<ui color="#000000">
  <image id="face" size="-1,-1" fit="contain" playback="segmented"
    src="gif/smile_[0.5s].gif">
    <text id="caption" anchor="0,1" pivot="0,1" size="-1,120"
      position="0,-24" margin="40,0,40,0" visible="false" font-size="28"
      align="center" valign="bottom" wrap="true" />
  </image>
</ui>
```

Image 继承 Block 的坐标空间，子节点以它的完整布局矩形为参考，先绘制图片再绘制文字。`contain` 的留白不会缩放子节点，圆角也不裁子树。所有文字共用 `caption`，背景完全透明；文字为空时自动隐藏。提示和回答会相互替换，不会分成上下两行独立显示。

项目只保存 23 个文件名和坐标。启动只解码初始 `src`；切换时调用 `face.SetSource("gif/目标文件.gif")`，框架才读取目标。读取、校验成功后才替换，旧素材、播放画布和待完成回调不再保留；不会建立全部 GIF 的全局缓存。换源时，原有文字组件不重建、不清空。

`SetSource` 成功后从第一帧播放，同一路径也重新加载。需要自然回正时先调用 `Finish`，在完成回调中换源；需要立即切换时直接 `SetSource`。组件更新放在窗口主循环，HTTP 请求使用 `app.Dispatch` 提交。

## 表情坐标和分段

输入顺序是 `[情绪值, 唤醒度]`，使用 **5×5 矩阵**，两项均为 **0～4** 的整数，中心是 `[2,2]`：

| 维度 | 0 | 2 | 4 |
| --- | --- | --- | --- |
| `valence` | 消极 | 中性 | 积极 |
| `arousal` | 平静、困倦 | 普通活跃程度 | 激动、兴奋 |

`internal/robot/emotion.go` 给每张 GIF 登记坐标，计算 `dx*dx + dy*dy` 选择最近的一张；等距时采用登记顺序。坐标是项目自己的可调配置，不是参考 PDF 中的精确数值。23 张素材各占一个唯一格子，另外两格按最近距离选择；坐标不是把旧 0～10 直接取整，避免多张表情挤在同一格。新版接口不再接受大于 4 的输入。

| 唤醒度 \ 情绪值 | 0 | 1 | 2 | 3 | 4 |
| --- | --- | --- | --- | --- | --- |
| 4 | 惊恐 | 生气 | 惊讶 | 运动 | 开心 |
| 3 | 哭泣 | 调皮 | 疑惑 | 音乐 | 坏笑 |
| 2 | 难过 | 思考 | 正常 | 观察 | 自信 |
| 1 | 叹气 | 就近 | 静音 | 害羞 | 美味 |
| 0 | 就近 | 困倦 | 睡觉 | 悠闲 | 微笑 |

| 文件名参数 | 进入与保持/循环 | 请求结束后 |
| --- | --- | --- |
| `[0.5s]` | 播放到 0.5 秒后保持 | 从保持位置继续到末尾 |
| `[0.5s,1.5s]` | 前 0.5 秒进入，循环 0.5～1.5 秒 | 完成当前轮后播放回正段 |
| `[29]` | 播放到第 29 帧后保持 | 从该位置继续 |
| `[29,50]` | 29 之前进入，循环 29～50 帧，含末帧 | 完成当前轮后回正 |

帧索引从 0 开始，第二个参数是结束位置，不是时长。支持逗号周围空格及 `2.0 s` 写法；原始帧延迟不变。仍在进入段时收到切换请求，会先完成进入再接回正。像睡觉的 21.5 秒进入段较长，过早切换也会等到它完成。

同一张表情重复选中会继续播放。回正期间只保留最新目标，不积压旧目标；目标改回当前表情时，也先完成回正再重新进入。没有新请求时，单值保持、双值循环。普通图片默认 `playback="auto"`，不解析文件名分段。

“思考”原素材名 `ponder_[0.5s].gif` 按规范会在 0.5 秒暂停，因此后续动作只在退出时播放。本项目将它重命名为 `ponder_[0s,1.52s].gif`，使用完整 1.52 秒循环；GIF 内容和原帧延迟不变，首尾相同，切换时完成当前一轮后进入目标表情。其它单值素材仍按规范暂停，不将所有暂停一概当成卡顿。

## 表情接口

```http
POST /api/emotion
Content-Type: application/json

{"valence":4,"arousal":4}
```

`GET /api/emotion` 和成功的 POST 都返回：

| 字段 | 含义 |
| --- | --- |
| `current.vector` | 最新接收的坐标 |
| `current.expression` | 当前实际显示的表情 |
| `current.target` | 最新目标表情 |
| `current.transitioning` | 是否等待当前表情回正 |
| `expressions` | 全部素材的名称、标签、坐标和文件名 |
| `grid_size` | 每条轴的档数，固定为 5 |

POST 成功表示目标已提交，不等待整段回正完成；轮询 GET 可确认实际显示状态。大模型控制例子：

```sh
curl -X POST http://127.0.0.1:8080/api/emotion \
  -H 'Content-Type: application/json' \
  -d '{"valence":4,"arousal":4}'
```

## 文字接口

`POST /api/display` 更新唯一的文字区域，不会强制改变表情。`GET /api/display` 读取当前文字。

| 字段 | 用法 |
| --- | --- |
| `text` | 替换全部文字；空字符串清空并隐藏 |
| `append` | 将新片段追加到当前文字，供模型增量输出使用 |

两项必须且只能提供一项。提示、回答和其它文字都使用这两个字段，不另外定义状态字段。累计文字最多 8192 个 Unicode 字符，POST 请求体最多 64 KiB。

需要提示时直接发送普通文字：

```json
{"text":"正在倾听…"}
```

```json
{"text":"正在思考…"}
```

开始回答时，用第一段回答替换之前的提示：

```json
{"text":"你好，"}
```

后续片段按生成顺序追加，等待上一条请求成功后再发送下一条：

```json
{"append":"今天天气很好。"}
```

回答结束后保留当前文字，不需要额外请求。要清空时发送：

```json
{"text":""}
```

成功响应示例：

```json
{"current":{"text":"你好，今天天气很好。"}}
```

这是连续提交片段的 HTTP 接口，项目不接入具体大模型服务，不自动生成文字。并行客户端按窗口主循环实际接收的顺序追加；单次回答应由调用方按生成顺序发送。长文字在底部区域显示末尾几行，以便看到最新输出；完整内容可通过 GET 或 Web 面板读取。切换表情不会清空文字。

Windows PowerShell 例子：

```powershell
Invoke-RestMethod -Method Post `
  -Uri 'http://127.0.0.1:8080/api/display' `
  -ContentType 'application/json; charset=utf-8' `
  -Body '{"text":"你好，我在听。"}'
```

两类接口的错误响应均为 `{"error":"错误说明"}`：输入错误返回 `400`，窗口不可用返回 `503`，等待主循环超过 3 秒返回 `504`。非法输入保留原内容。

## Web 调试

- 主页面只有实际屏幕、5×5 表情矩阵和文字控制，不重复列一份全部表情。
- 预览由 `GET /api/preview` 获取窗口最近绘制的真实帧，包含表情分段、回正和字幕；浏览器不另外播放原始 GIF。
- 前端完整解码新帧后覆盖同一个 Canvas，失败保留上一帧，切换动作不刷新页面、不清空画布。
- 最多约 20 次/秒串行取帧，预览宽度最多 640px、JPEG 品质 85；保持窗口画面比例。与设备使用同一播放进度，但受采样和网络影响有少量延迟，并不保证逐帧、同一时刻完全一致。
- 设备状态每 0.5 秒读取一次，文字状态每秒读取。页面隐藏时暂停预览取帧，关闭网页后不再有预览开销。
- 设备上只有一个底部 Text，背景完全透明，内容为空时隐藏。
- 额外的增量追加、模拟流式回答收在“增量输出测试”里；模拟仍按每段 8 字符顺序提交。
- 文字输入框保留未提交的草稿；追加片段或后台轮询不会覆盖草稿。模拟流式输出失败时保留完整原文，便于再次发送。提交全文、清空或流式输出成功后，再与设备结果同步。
- 网页编进程序，不使用外部 CDN 或前端构建工具。

`Application.Snapshot(ctx)` 是框架的通用截图能力，复制已绘制的 RGBA 帧，不重新推进动画。它内部会调用 Dispatch，只在 HTTP 等外部 goroutine 中调用；不要再套一层 Dispatch。预览缩放、JPEG 编码和 Canvas 控件都留在机器人项目，框架不包含 Web 协议。

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
