# web-socket
web socket的实现  
## 使用  
```
// 服务端在http.Handler函数中调用Accept()
func(writer http.ResponseWriter, request *http.Request) {
    conn, err := Accept(writer, request, "text", nil)
    for {
        conn.Read()
        conn.Write()
    }
}
// 户口端调用Dial() 
conn, err := Dial(request *http.Request, conn net.Conn, "text", nil)
for {
    conn.Write()
    conn.Read()
}
```
## 处理控制帧
web socket要求随时可以处理控制帧，一个消息被分片发送的过程中，可以插入一个控制帧，然后继续发送剩余的分片。  
然而，控制帧也可以有数据，调用io.Reader接口，无法区别读取的数据，是数据帧还是控制帧。 
如果不想处理回调，可以使用Reader.ReadData()来读取对方的一段完整的数据，并通过返回判断。 
```
code, data, err := reader.ReadData()
if err != nil {
    return 0, err
}
if IsCtrlCode(code) {
// 控制帧
}else {
// 数据帧
}
``` 
