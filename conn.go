package socket

import (
	"net"
)

func NewBinaryConn(conn net.Conn, mask bool, handler HandleControlFragment) *Conn {
	return newConn(conn, CodeBinary, mask, handler)
}

func NewTextConn(conn net.Conn, mask bool, handler HandleControlFragment) *Conn {
	return newConn(conn, CodeText, mask, handler)
}

func newConn(conn net.Conn, code Code, mask bool, handler HandleControlFragment) *Conn {
	p := new(Conn)
	p.c = conn
	p.Writer = NewWriter(conn, code, mask)
	if handler == nil {
		p.Reader = NewReader(conn, p.HandleControlFragment)
	} else {
		p.Reader = NewReader(conn, handler)
	}
	return p
}

type Conn struct {
	*Writer
	*Reader
	c net.Conn
}

func (c *Conn) Close() error {
	// 先发close控制帧
	_, err := c.WriteClose(nil)
	if err != nil {
		c.c.Close()
		return err
	}
	// 关闭底层
	return c.c.Close()
}

func (c *Conn) HandleControlFragment(code Code, data []byte) error {
	switch code {
	case CodePing:
		_, err := c.WritePong(data)
		return err
	case CodeClose:
		return c.Close()
	default:
		return nil
	}
}
