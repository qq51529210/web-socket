package websocket

import (
	"io"
	"net/http"
	"testing"
)

func Test_websocket(t *testing.T) {
	var ser http.Server
	ser.Addr = "127.0.0.1:3390"
	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		io.WriteString(rw, `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Test WebSocket</title>
	<script type="text/javascript">
		function append(d) {
			var output = document.getElementById('output')
			var p = document.createElement('p')
			p.innerHTML += d
			output.appendChild(p)
		}
		var ws = new WebSocket("ws://127.0.0.1:3390/ws")
		ws.onopen = function(e) {
			append('open')
		}
		ws.onmessage = function(e){
			append(e.data)
		}
		ws.onclose = function(e){
			append('close')
		}
		ws.onerror = function(e){
			append(e.data)
		}
		function send() {
			ws.send(document.getElementById('input').value)
		}
		function clear() {
			document.getElementById('output').innerHTML=""
		}
	</script>
</head>
<body>
	<div>
		<input type="text" id="input" />
		<input type="button" value="send" onclick="send()" />
		<input type="button" value="clear" onclick="clear()" />
	</div>
	<div id="output">
		<p id="output"></p>
	</div>
</body>
</html>
`)
	})
	http.HandleFunc("/ws", func(rw http.ResponseWriter, r *http.Request) {
		payload := 1024
		conn, err := Accept(rw, r)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		err = conn.Write(CodeText, []byte("hello client."), payload)
		if err != nil {
			t.Fatal(err)
		}
		err = conn.ReadLoop(payload, func(c Code, b []byte) error {
			if string(b) == "close" {
				err = conn.Write(CodeClose, nil, payload)
				ser.Close()
				return err
			}
			return conn.Write(c, append([]byte("server:"), b...), payload)
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	ser.ListenAndServe()
}
