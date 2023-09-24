# IM

[English](./README.md) | 中文 IM 是一个用于高效管理 WebSocket 连接的 Go 语言库，可实现用户和设备之间的实时通信。这个库简化了 WebSocket 服务器的设置，并能够优雅地处理多个用户和设备。

## 主要特点

- 创建和管理多个用户池，每个池关联一个特定的用户群体。
- 将用户和设备添加到用户池，并优雅地处理断开连接。
- 使用消息分发器将消息分发给同一池内的用户，实现实时通信。
- 使用消息池有效地管理消息对象，以便复用。

## 安装

要在 Go 项目中使用 IM 库，请将其添加为依赖项到你的 `go.mod` 文件中：

```shell
go get github.com/0x2d3c/im
```

## 入门指南

以下是如何使用 IM 库的简要指南：

1. 在你的 Go 代码中导入 IM 库。

2. 创建一个 WebSocket 服务器，并将 `HandleWebSocket` 函数用作 WebSocket 升级请求的处理程序：

   ```go
   http.HandleFunc("/ws/v0", v0.HandleWebSocket)
   http.HandleFunc("/ws/v1", v1.HandleWebSocket)
   log.Fatal(http.ListenAndServe(":8080", nil))
   ```

3. 在你的客户端应用程序中，建立 WebSocket 连接并将用户和设备信息作为查询参数传递：

   ```javascript
   const userID = "123";
   const deviceID = "web";

   // 连接到 WebSocket 服务器
   const socket = new WebSocket(`ws://localhost:8080/ws/v0?user=${userID}&device=${deviceID}`);
   const socket = new WebSocket(`ws://localhost:8080/ws/v1?user=${userID}&device=${deviceID}`);

   // 处理传入的消息，并根据需要发送消息
   ```

4. 利用 IM 库的功能来管理用户连接并高效地分发消息。

5. 根据你的应用程序需求自定义 WebSocket 服务器。

## 消息结构

IM 使用以下 `Message` 结构来表示 WebSocket 消息：

```go
type Message struct {
    At        int64           // 消息的时间戳。
    Device    string          // 设备标识符。
    Sender    string          // 发送者的用户 ID。
    MBytes    json.RawMessage // 以 JSON 格式表示的消息内容。
    Receivers []string        // 消息接收者的用户 ID。
}
```

你可以在 `MBytes` 字段中发送包含 JSON 内容的消息，用于与其他用户进行通信。

## 贡献

如果你遇到任何问题或有改进建议，请随时提交问题或创建拉取请求。

## 许可证

IM 是一款开源软件，根据 MIT 许可证发布。有关完整的许可证详情，请参阅 [LICENSE](LICENSE) 文件。