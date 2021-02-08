package socket

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"time"
)

type Fin byte

const (
	Fin1  Fin  = 0x80
	Fin0  Fin  = 0x00
	mask1 byte = 0x80
	mask0 byte = 0x00
)

type Code byte

const (
	CodeContinuation Code = 0  // 分片帧
	CodeText         Code = 1  // 数据帧
	CodeBinary       Code = 2  // 数据帧
	CodeClose        Code = 8  // 控制帧
	CodePing         Code = 9  // 控制帧
	CodePong         Code = 10 // 控制帧
)

func IsCtrlCode(code Code) bool {
	switch code {
	case CodeClose, CodePing, CodePong:
		return true
	default:
		return false
	}
}

var codes = []Code{CodeContinuation, CodeText, CodeBinary, CodeClose, CodePing, CodePong}

func isCode(code byte) bool {
	for i := 0; i < len(codes); i++ {
		if byte(codes[i]) == code {
			return true
		}
	}
	return false
}

// 随机数
var _rand = rand.New(rand.NewSource(time.Now().UnixNano()))

// 创建一个web socket服务端的连接
// res，req:http标准库Handler的参数
// res自动设置了必须字段，其他字段必须在调用前设置，因为里面调用了res.WriteHeader()
// _type:表示生成什么帧类型，text/binary
// ctrl:处理控制帧回调，nil使用默认处理
func Accept(res http.ResponseWriter, req *http.Request, _type string, ctrl HandleControlFragment) (*Conn, error) {
	// 检查请求头
	accept, err := CheckRequestHeader(req.Header)
	if nil != err {
		return nil, err
	}
	// 设置响应头
	SetResponseHeader(res.Header(), accept)
	// 状态码
	res.WriteHeader(http.StatusSwitchingProtocols)
	// 获取底层的连接对象
	conn, err := Hijacker(res)
	if nil != err {
		return nil, err
	}
	// 服务端不需要mark
	if _type == "binary" {
		return NewTextConn(conn, false, ctrl), nil
	}
	return NewBinaryConn(conn, false, ctrl), nil
}

// 创建一个web socket客户端的连接
// req:请求头，必须字段自动设置，其他字段，调用前设置
// conn:底层连接，可以是tls.Conn
// _type:表示生成什么帧类型,text/binary
// ctrl:处理控制帧回调，nil使用默认处理
func Dial(req *http.Request, conn net.Conn, _type string, ctrl HandleControlFragment) (*Conn, error) {
	// 设置相关的请求头
	key := SetRequestHeader(req, conn.RemoteAddr().String())
	// 发送
	err := req.Write(conn)
	if err != nil {
		return nil, err
	}
	// 读取响应
	res, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return nil, err
	}
	err = CheckResponseHeader(res.StatusCode, res.Header, key)
	if err != nil {
		return nil, err
	}
	// 客户端必须mark
	if _type == "binary" {
		return NewTextConn(conn, true, ctrl), nil
	}
	return NewBinaryConn(conn, true, ctrl), nil
}

// 获取底层的连接对象
func Hijacker(res http.ResponseWriter) (net.Conn, error) {
	h, o := res.(http.Hijacker)
	if !o {
		return nil, errors.New("hijacker response conn fail")
	}
	conn, buf, err := h.Hijack()
	if nil != err {
		return nil, err
	}
	// 响应给客户端
	err = buf.Flush()
	if nil != err {
		return nil, err
	}
	return conn, nil
}

// 设置必须的请求头，返回Sec-WebSocket-Key
func SetRequestHeader(req *http.Request, host string) string {
	sec_websocket_key := GenSecWebSocketKey()
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", sec_websocket_key)
	req.Header.Set("Host", host)
	return sec_websocket_key
}

// 设置必须的响应头
func SetResponseHeader(header http.Header, secWebSocketAccept string) {
	header.Set("Upgrade", "websocket")
	header.Set("Connection", "Upgrade")
	header.Set("Sec-WebSocket-Accept", secWebSocketAccept)
}

// 检查请求头必须的选项，返回Sec-WebSocket-Accept的值
func CheckRequestHeader(header http.Header) (string, error) {
	for _, check := range []func(http.Header) error{
		CheckUpgradeHeader,
		CheckConnectionHeader,
		CheckSecWebSocketVersionHeader,
	} {
		err := check(header)
		if err != nil {
			return "", err
		}
	}
	return CheckSecWebSocketKey(header)
}

// 检查响应头必须的选项
func CheckResponseHeader(statusCode int, header http.Header, secWebSocketKey string) error {
	// 状态吗101
	if http.StatusSwitchingProtocols != statusCode {
		return errors.New(fmt.Sprintf("invalid response status code '%d'", statusCode))
	}
	// 响应头
	for _, check := range []func(http.Header) error{
		CheckUpgradeHeader,
		CheckConnectionHeader,
	} {
		err := check(header)
		if err != nil {
			return err
		}
	}
	// 返回的Sec-WebSocket-Accept
	return CheckSecWebSocketAccept(header, GenSecWebSocketAccept(secWebSocketKey))
}

// 检查必须的选项
func CheckUpgradeHeader(header http.Header) error {
	v, o := header["Upgrade"]
	if o {
		for _, s := range v {
			if s == "websocket" {
				return nil
			}
		}
	}
	return fmt.Errorf("header 'Upgrade' must contain 'websocket'")
}

// 检查必须的选项
func CheckConnectionHeader(header http.Header) error {
	v, o := header["Connection"]
	if o {
		for _, s := range v {
			if s == "Upgrade" {
				return nil
			}
		}
	}
	return fmt.Errorf("header 'Connection' must contain 'Upgrade'")
}

// 检查必须的选项
func CheckSecWebSocketVersionHeader(header http.Header) error {
	//http.CanonicalHeaderKey("Sec-WebSocket-Version")
	v, o := header["Sec-Websocket-Version"]
	if o {
		if len(v) == 1 && v[0] == "13" {
			return nil
		}
	}
	return fmt.Errorf("header 'Sec-Websocket-Version' must be '13'")
}

// 检查必须的选项
func CheckSecWebSocketKey(header http.Header) (string, error) {
	v, o := header["Sec-Websocket-Key"]
	if !o {
		return "", fmt.Errorf("header 'Sec-WebSocket-Key' must be set")
	}
	if len(v) != 1 || v[0] == "" {
		return "", fmt.Errorf("header 'Sec-WebSocket-Key' invalid")
	}
	return GenSecWebSocketAccept(v[0]), nil
}

// 检查必须的选项，计算并返回Sec-WebSocket-Accept的值
func CheckSecWebSocketAccept(header http.Header, key string) error {
	v, o := header["Sec-Websocket-Accept"]
	if !o {
		return fmt.Errorf("header 'Sec-WebSocket-Accept' must be set")
	}
	if len(v) != 1 || v[0] != key {
		return fmt.Errorf("header 'Sec-WebSocket-Accept' invalid")
	}
	return nil
}

// 生成随机的Sec-WebSocket-Key
func GenSecWebSocketKey() string {
	var b [16]byte
	_rand.Read(b[:])
	return base64.StdEncoding.EncodeToString(b[:])
}

// 根据Sec-WebSocket-Key生成Sec-WebSocket-Accept
func GenSecWebSocketAccept(webSocketKey string) string {
	hash := sha1.New()
	hash.Write([]byte(webSocketKey + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}
