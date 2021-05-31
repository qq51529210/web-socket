package websocket

import (
	"encoding/binary"
	"fmt"
	"io"
)

type Code byte

func (c Code) String() string {
	switch c {
	case CodeText:
		return "text"
	case CodeBinary:
		return "binary"
	case CodeClose:
		return "close"
	case CodePing:
		return "ping"
	case CodePong:
		return "pong"
	default:
		return "continuation"
	}
}

const (
	codeContinuation Code = 0
	CodeText         Code = 1
	CodeBinary       Code = 2
	CodeClose        Code = 8
	CodePing         Code = 9
	CodePong         Code = 10
)

var (
	_Fin  = [2]byte{0x00, 0x80}
	_Mask = [2]byte{0x00, 0x80}
)

type Conn struct {
	reader io.Reader
	writer io.Writer
	mask   byte
}

// Write code type data.
// If data length bigger than payload, it will be split into multiple frames.
func (c *Conn) Write(code Code, data []byte, payload int) error {
	if len(data) <= payload {
		return c.writeFrame(_Fin[1], code, data)
	}
	switch code {
	case CodeBinary, CodeText:
	default:
		return fmt.Errorf(`could not split "%s" data into frames`, code.String())
	}
	// If length of data bigger than c.maxSize[1] split into frames.
	p := data
	// First frame, fin=0, code!=0.
	err := c.writeFrame(_Fin[0], code, p[:payload])
	if err != nil {
		return err
	}
	p = p[payload:]
	// Continuation frame, fin=0, code=0.
	for len(p) > payload {
		err := c.writeFrame(_Fin[0], codeContinuation, p[:payload])
		if err != nil {
			return err
		}
		p = p[payload:]
	}
	// The last frame, fin=1, opcode=0.
	return c.writeFrame(_Fin[1], codeContinuation, p)
}

// Write a frame.
func (c *Conn) writeFrame(fin byte, code Code, data []byte) error {
	b := buffPool.Get().(*buffer)
	b.n = 0
	// Encode fin and code.
	b.Put8(fin | byte(code))
	// Encode mask and payload length.
	payload := len(data)
	if payload < 126 {
		b.Put8(c.mask | byte(payload))
	} else if payload <= 0xffff {
		b.Put8(c.mask | 126)
		b.Put16(payload)
	} else {
		b.Put8(c.mask | 127)
		b.Put64(payload)
	}
	// Encode mask key.
	if c.mask != 0 {
		key := b.b[b.n:]
		b.PutRand(4)
		// Append mask data.
		b.PutMask(key, data)
		// Write buffer.
		_, err := c.writer.Write(b.b[:b.n])
		buffPool.Put(b)
		return err
	}
	// Write header.
	_, err := c.writer.Write(b.b[:b.n])
	if err != nil {
		buffPool.Put(b)
		return err
	}
	buffPool.Put(b)
	// Write data.
	_, err = c.writer.Write(data)
	return err
}

// Read a complete message then call handle.
// If message length bigger than maxLen, it return error.
func (c *Conn) ReadLoop(maxLen int, handle func(Code, []byte) error) error {
	var (
		fin     byte
		code    Code
		mask    byte
		length  int
		err     error
		header  [8]byte
		payload [16][]byte
	)
	for {
		// Read a frame header.
		_, err = io.ReadFull(c.reader, header[:2])
		if err != nil {
			return err
		}
		// Fin
		fin = header[0] & _Fin[1]
		// code
		code = Code(header[0] & 0x0f)
		// mask
		mask = header[1] & _Mask[1]
		// length
		ch := header[1] & 0x7f
		switch ch {
		case 126:
			// 2 bytes.
			_, err = io.ReadFull(c.reader, header[:2])
			if err != nil {
				return err
			}
			length = int(binary.BigEndian.Uint16(header[:]))
		case 127:
			// 8 bytes.
			_, err = io.ReadFull(c.reader, header[:8])
			if err != nil {
				return err
			}
			length = int(binary.BigEndian.Uint64(header[:]))
		default:
			// 1 bytes.
			length = int(ch)
		}
		// key
		if mask != 0 {
			_, err = io.ReadFull(c.reader, header[:4])
			if err != nil {
				return err
			}
		}
		// Read a frame payload.
		i := len(payload[code])
		if cap(payload[code]) < length+i {
			payload[code] = append(payload[code], make([]byte, length-(cap(payload[code])-i))...)
		} else {
			payload[code] = payload[code][:length+len(payload[code])]
		}
		_, err = io.ReadFull(c.reader, payload[code][i:])
		if err != nil {
			return err
		}
		// Mask payload data.
		if mask != 0 {
			key := header[:4]
			for i := 0; i < len(payload[code]); i++ {
				payload[code][i] ^= key[i%4]
			}
		}
		// Last frame of a message
		if fin == _Fin[1] {
			// Call back handle.
			handle(code, payload[code])
			// Reset buffer.
			payload[code] = payload[code][:0]
		}
	}
}
