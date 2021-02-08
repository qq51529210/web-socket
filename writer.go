package socket

import (
	"bytes"
	"encoding/binary"
	"io"
)

func NewWriter(writer io.Writer, code Code, mask bool) *Writer {
	p := new(Writer)
	p.writer = writer
	p.code = code
	if mask {
		p.mask.mask = mask1
	} else {
		p.mask.mask = mask0
	}
	return p
}

type Writer struct {
	mask    Mask         // 掩码
	buffer  bytes.Buffer // 缓存
	writer  io.Writer    // 输出
	code    Code         // 数据帧的类型
	payload int          // 发送数据分片大小
	str     bytes.Buffer // WriteString的缓存
}

// wb缓存扩容，返回，原来的大小
func (this *Writer) growWB(n int) []byte {
	i := this.buffer.Len()
	this.buffer.Grow(i + n)
	return this.buffer.Bytes()[i:]
}

// 发送数据，如果len(data)>payload length，自动分片
func (this *Writer) Write(data []byte) (int, error) {
	// 控制帧/数据太小，不分片，直接发fin=1,opcode!=0
	if len(data) <= this.payload || this.payload < 1 {
		return this.WriteFragment(Fin1, this.code, data)
	}
	// 把数据分片发
	p := data
	var n int
	// 1.第一帧，发fin=0,opcode!=0
	n, err := this.WriteFragment(Fin0, this.code, p[:this.payload])
	if err != nil {
		return n, err
	}
	n += this.payload
	p = p[this.payload:]
	// 2.中间帧，发fin=0,opcode=0
	for len(p) > this.payload {
		_, err = this.WriteFragment(Fin0, CodeContinuation, p[:this.payload])
		if err != nil {
			return n, err
		}
		n += this.payload
		p = p[this.payload:]
	}
	// 3.最后一帧，发fin=1,opcode=0
	_, err = this.WriteFragment(Fin1, CodeContinuation, p)
	if err != nil {
		return n, err
	}
	n += len(p)
	return n, nil
}

// 发送字符串数据，先缓存成[]byte，然后在调用Write()
func (this *Writer) WriteString(data string) (int, error) {
	this.str.Reset()
	this.str.WriteString(data)
	return this.Write(this.str.Bytes())
}

// 发送一个完整的数据帧
// 数据分帧步骤：看Write()的实现
// 1.第一帧，fin=Fin0,code!=CodeContinuation
// 2.中间帧，fin=Fin0,code=CodeContinuation
// 3.后一帧，fin=Fin1,code=CodeContinuation
func (this *Writer) WriteFragment(fin Fin, code Code, data []byte) (int, error) {
	this.buffer.Reset()
	// fin + code
	this.buffer.WriteByte(byte(fin) | byte(code))
	// mask +payload length
	payload := uint64(len(data))
	if payload < 126 {
		// 如果 0-125，占[1]的剩下7bits
		this.buffer.WriteByte(this.mask.mask | uint8(payload))
	} else if payload <= 0xffff {
		// 如果 126-0xffff，使用[2-3]2字节表示，把[1]剩下7bits设为126
		this.buffer.WriteByte(this.mask.mask | 126)
		binary.BigEndian.PutUint16(this.growWB(2), uint16(payload))
	} else {
		// 如果 0xffff ~，使用[2-9]8字节表示，把[1]剩下7bits设为127
		this.buffer.WriteByte(this.mask.mask | 127)
		binary.BigEndian.PutUint64(this.growWB(8), payload)
	}
	// if mask
	if this.mask.mask == mask1 {
		// key
		this.buffer.Write(this.mask.InitKey())
		// data
		i := this.buffer.Len()
		this.buffer.Write(data)
		// mask data
		this.mask.ResetIndex()
		this.mask.Mask(this.buffer.Bytes()[i:])
		// header + payload
		_, err := this.writer.Write(this.buffer.Bytes())
		return len(data), err
	}
	// header
	n, err := this.writer.Write(this.buffer.Bytes())
	if err != nil {
		return n, err
	}
	// payload
	return this.writer.Write(data)
}

// 发送ping控制帧
func (this *Writer) WritePing(data []byte) (int, error) {
	return this.WriteFragment(Fin1, CodePing, data)
}

// 发送ping控制帧
func (this *Writer) WritePong(data []byte) (int, error) {
	return this.WriteFragment(Fin1, CodePong, data)
}

// 发送close控制帧
func (this *Writer) WriteClose(data []byte) (int, error) {
	return this.WriteFragment(Fin1, CodeClose, data)
}

// 设置发送数据分片payload length的最大值
func (this *Writer) SetPayload(n int) {
	this.payload = n
}
