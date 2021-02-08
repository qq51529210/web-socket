package socket

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestMask_Mask(t *testing.T) {
	b1 := []byte("test Mask")
	b2 := make([]byte, len(b1))
	copy(b2, b1)
	//
	m1 := new(Mask)
	m1.InitKey()
	m1.ResetIndex()
	m1.Mask(b2)
	if bytes.Equal(b2, b1) {
		t.FailNow()
	}
	//
	m2 := new(Mask)
	copy(m2.key[:], m1.key[:])
	m2.ResetIndex()
	m2.Mask(b2)
	if !bytes.Equal(b2, b1) {
		t.FailNow()
	}
}

func testReadWrite(t *testing.T, r io.Reader, w io.Writer) {
	data := []byte("hello")
	buf := make([]byte, len(data))
	// 发送数据
	n, err := w.Write(data)
	if err != nil {
		t.Error(err)
	}
	if len(data) != n {
		t.FailNow()
	}
	// 接受数据
	n, err = r.Read(buf)
	if err != nil {
		t.Error(err)
	}
	if len(data) != n {
		t.FailNow()
	}
	//
	if !bytes.Equal(buf, data) {
		t.FailNow()
	}
}

func Test_Read_Writer(t *testing.T) {
	conn := bytes.NewBuffer(nil)
	// 文本
	testReadWrite(t, NewReader(conn, nil), NewWriter(conn, CodeText, true))
	// 二进制
	testReadWrite(t, NewReader(conn, nil), NewWriter(conn, CodeBinary, true))
}

func Test_HandleControlFragment(t *testing.T) {
	buf := make([]byte, 32)
	conn := bytes.NewBuffer(nil)
	w := NewWriter(conn, CodeText, true)
	r := NewReader(conn, func(code Code, data []byte) error {
		var err error
		switch code {
		case CodePing:
			_, err = w.WritePong(append(data, []byte("-pong")...))
		case CodePong:
			_, err = w.WriteClose(append(data, []byte("-close")...))
		default:
			_, err = w.Write(append(data, []byte("-ok")...))
		}
		return err
	})
	// ping
	_, err := w.WritePing([]byte("ping"))
	if err != nil {
		t.Fatal(err)
	}
	// 读取数据
	n, err := r.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal([]byte("ping-pong-close-ok"), buf[:n]) {
		t.FailNow()
	}
}

type testAccept struct {
	*bufio.ReadWriter
	http.Response
	bytes.Buffer
}

func newTestAccept() *testAccept {
	p := new(testAccept)
	p.ReadWriter = bufio.NewReadWriter(
		bufio.NewReader(&p.Buffer),
		bufio.NewWriter(&p.Buffer))
	p.Response.Header = make(http.Header)
	return p
}

func (this *testAccept) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, this.ReadWriter, nil
}

func (this *testAccept) Header() http.Header {
	return this.Response.Header
}

func (this *testAccept) Write(data []byte) (int, error) {
	return 0, nil
}

func (this *testAccept) WriteHeader(statusCode int) {
	this.Response.StatusCode = statusCode
}

type testDial struct {
	net.Conn
	buf bytes.Buffer
	ok  bool
}

func (this *testDial) Read(b []byte) (n int, err error) {
	if this.ok {
		return this.buf.Read(b)
	}
	this.ok = true
	request, err := http.ReadRequest(bufio.NewReader(&this.buf))
	if err != nil {
		return 0, err
	}
	CheckRequestHeader(request.Header)
	response := http.Response{
		StatusCode: 101,
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
	}
	SetResponseHeader(response.Header, GenSecWebSocketAccept(
		request.Header.Get(http.CanonicalHeaderKey("sec-WebSocket-Key"))))
	response.Write(&this.buf)
	return this.buf.Read(b)
}

func (this *testDial) Write(b []byte) (n int, err error) {
	return this.buf.Write(b)
}

func (this *testDial) Close() error {
	return nil
}

func (this *testDial) LocalAddr() net.Addr {
	return this
}

func (this *testDial) RemoteAddr() net.Addr {
	return this
}

func (this *testDial) SetDeadline(t time.Time) error {
	return nil
}

func (this *testDial) SetReadDeadline(t time.Time) error {
	return nil
}

func (this *testDial) SetWriteDeadline(t time.Time) error {
	return nil
}

func (this *testDial) Network() string {
	return "tcp"
}

func (this *testDial) String() string {
	return "127.0.0.1:80"
}

func Test_Accept(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:80", nil)
	if err != nil {
		t.Fatal(err)
	}
	SetRequestHeader(request, "127.0.0.1:80")
	_, err = Accept(newTestAccept(), request, "text", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_Dial(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:80", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Dial(request, new(testDial), "text", nil)
	if err != nil {
		t.Fatal(err)
	}
}
