English | [中文](./README.zh.md)
# IM

IM is a Go library for efficiently managing WebSocket connections, enabling real-time communication between users and devices. This library simplifies WebSocket server setup and handles multiple users and devices gracefully.

## Key Features

- Create and manage multiple user pools, each associated with a specific user group.
- Add users and devices to user pools and handle disconnections gracefully.
- Distribute messages to users within the same pool using a message dispatcher, enabling real-time communication.
- Efficiently manage message objects using a message pool for reuse.

## Installation

To use the IM library in your Go project, add it as a dependency to your `go.mod` file:

```shell
go get github.com/0x2d3c/im
```

## Getting Started

Here's a quick guide on how to use the IM library:

1. Import the IM library into your Go code.

2. Create a WebSocket server and use the `HandleWebSocket` function as the handler for WebSocket upgrade requests:

   ```go
   http.HandleFunc("/ws/v0", v0.HandleWebSocket)
   http.HandleFunc("/ws/v1", v1.HandleWebSocket)
   log.Fatal(http.ListenAndServe(":8080", nil))
   ```

3. In your client application, establish a WebSocket connection and pass user and device information as query parameters:

   ```javascript
   const userID = "123";
   const deviceID = "web";

   // Connect to the WebSocket server
   const socketV0 = new WebSocket(`ws://localhost:8080/ws/v0?user=${userID}&device=${deviceID}`);
   const socketV1 = new WebSocket(`ws://localhost:8080/ws/v1?user=${userID}&device=${deviceID}`);

   // Handle incoming messages and send messages as needed
   ```

4. Utilize the features of the IM library to manage user connections and distribute messages efficiently.

5. Customize the WebSocket server according to your application's requirements.

## Message Structure

IM uses the following `Message` structure to represent WebSocket messages:

```go
type Message struct {
    At        int64           // Timestamp of the message.
    Device    string          // Device identifier.
    Sender    string          // User ID of the sender.
    MBytes    json.RawMessage // Message content in JSON format.
    Receivers []string        // User IDs of message receivers.
}
```

You can send messages with JSON content in the `MBytes` field to communicate with other users.

## Contribution

If you encounter any issues or have suggestions for improvements, please feel free to submit issues or create pull requests.

## License

IM is open-source software released under the MIT License. For full license details, please refer to the [LICENSE](LICENSE) file.