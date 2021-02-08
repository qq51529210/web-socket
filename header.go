package socket

import (
	"encoding/binary"
	"fmt"
	"io"
)

const maxHeaderLen = 14 // 帧header最大缓存

// 帧头
type Header struct {
	buf  [maxHeaderLen]byte
	fin  Fin
	code Code
	mask Mask
	data uint64
}

func (h *Header) Decode(r io.Reader) (err error) {
	_, err = io.ReadFull(r, h.buf[:2])
	if err != nil {
		return
	}
	// fin
	h.fin = Fin(h.buf[0] & 0x80)
	// code
	code := h.buf[0] & 0x0f
	if !isCode(code) {
		return fmt.Errorf("invalid code value %d", code)
	}
	h.code = Code(code)
	// mask
	h.mask.mask = h.buf[1] & 0x80
	// payload length
	h.data = uint64(h.buf[1] & 0x7f)
	switch h.data {
	case 126:
		// 接下来2字节表示长度
		_, err = io.ReadFull(r, h.buf[:2])
		if err != nil {
			return
		}
		h.data = uint64(binary.BigEndian.Uint16(h.buf[:2]))
	case 127:
		// 接下来8字节表示长度
		_, err = io.ReadFull(r, h.buf[:8])
		if err != nil {
			return
		}
		h.data = binary.BigEndian.Uint64(h.buf[:8])
	default:
		// 小于126
	}
	// key
	if h.mask.mask == mask1 {
		_, err = io.ReadFull(r, h.mask.key[:4])
		// 重置下标
		h.mask.ResetIndex()
	}
	return
}

func (h *Header) IsComplete() bool {
	return h.fin == Fin1 && h.code != CodeContinuation
}
