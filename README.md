# web-socket

A Goland development package of web socket.

## Usage
Create a new connection.
```go
// Serve side call Accept()
func(res http.ResponseWriter, req *http.Request) {
    // Accept a new connection.
    conn, err := Accept(res, req)
    // Read message loop.
    conn.ReadLoop(max, handle(code, data){
        // Handle data and response.
        conn.Write(code, data, payload)
    })
}

// Client side call Dial(). 
conn, err := Dial()
conn.Write(code, data, payload)
// Read message loop.
conn.ReadLoop(max, handle(code, data){
    // Handle data and response.
    conn.Write(code, data, payload)
})
```
