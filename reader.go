package socket

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

const (
	defaultReadBuffer = 64 * 1024 // 默认的读缓存
)

type HandleControlFragment func(Code, []byte) error

func NewReader(r io.Reader, h HandleControlFragment) *Reader {
	p := new(Reader)
	p.reader = bufio.NewReaderSize(r, defaultReadBuffer)
	p.handler = h
	if p.handler == nil {
		p.handler = p.HandleControlFragment
	}
	p.dataFin = Fin1
	return p
}

type Reader struct {
	reader   *bufio.Reader         // 读缓存
	dataBuf  bytes.Buffer          // 数据帧的数据
	dataFin  Fin                   // Fin0，正在读
	dataCode Code                  // 当前正在读的数据帧的类型
	ctrlBuf  bytes.Buffer          // 控制帧的数据
	header   Header                // 帧头缓存
	handler  HandleControlFragment // 处理控制帧回调
}

func (r *Reader) HandleControlFragment(code Code, data []byte) error {
	return nil
}

// 读取payload的数据
func (r *Reader) Read(buf []byte) (int, error) {
	if r.dataBuf.Len() < 1 || r.dataFin != Fin1 {
		for {
			code, data, err := r.ReadData()
			if err != nil {
				return 0, err
			}
			if IsCtrlCode(code) {
				err = r.handler(code, data)
				if err != nil {
					return 0, err
				}
				continue
			}
			break
		}
	}
	return r.dataBuf.Read(buf)
}

var UnorderedFragmentError = errors.New("unordered fragment")

// 读取完整一帧的数据
// code:帧类型
// data:payload数据，属于Reader的内部的缓存（下一次会变），保存应该deep copy
func (r *Reader) ReadData() (code Code, data []byte, err error) {
	for {
		// 解析帧头
		err = r.header.Decode(r.reader)
		if err != nil {
			return
		}
		// 是控制帧
		if IsCtrlCode(r.header.code) {
			// 控制帧不能分片
			if r.header.fin != Fin1 {
				err = fmt.Errorf("invalid control fragment fin %d code %d", r.header.fin, r.header.code)
				return
			}
			// 读数据
			r.ctrlBuf.Reset()
			err = r.readData(&r.ctrlBuf)
			if err != nil {
				return
			}
			code = r.header.code
			data = r.ctrlBuf.Bytes()
			return
		}
		// 这个数据帧是否分帧，是哪一部分的帧
		// 完整帧，fin=Fin1,code!=CodeContinuation
		//
		// 1.第一帧，fin=Fin0,code!=CodeContinuation
		// 2.中间帧，fin=Fin0,code=CodeContinuation
		// 3.后一帧，fin=Fin1,code=CodeContinuation
		// reader的状态
		// 1.正在读分片，dataFin=Fin0,code!=CodeContinuation
		// 2.开始读，dataFin=Fin1,code=CodeContinuation
		if r.header.fin == Fin1 {
			if r.header.code != CodeContinuation {
				// 完整帧
				if r.dataFin != Fin1 {
					// 上一次数据没有读完
					err = UnorderedFragmentError
					return
				}
				code = r.header.code
			} else {
				// 分帧，最后一帧
				if r.dataFin != Fin0 {
					err = UnorderedFragmentError
				}
				r.dataFin = Fin1
				code = r.dataCode
			}
			// 读数据
			err = r.readData(&r.dataBuf)
			data = r.dataBuf.Bytes()
			// 完成了，返回
			return
		} else {
			if r.header.code != CodeContinuation {
				// 分帧，第一帧
				if r.dataFin != Fin1 {
					// 上一次数据没有读完
					err = UnorderedFragmentError
					return
				}
				r.dataFin = Fin1
				r.dataCode = code
			} else {
				// 分帧，中间帧
				if r.dataFin != Fin0 {
					// 没有收到第一帧
					err = UnorderedFragmentError
					return
				}
			}
			// 读数据
			err = r.readData(&r.dataBuf)
			if err != nil {
				return
			}
		}
	}
}

func (r *Reader) readData(buf *bytes.Buffer) error {
	i := buf.Len()
	_, err := io.CopyN(buf, r.reader, int64(r.header.data))
	if err != nil {
		return err
	}
	r.header.mask.Mask(buf.Bytes()[i:])
	return nil
}
