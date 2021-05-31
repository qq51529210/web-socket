package websocket

import "sync"

var (
	buffPool sync.Pool
)

func init() {
	buffPool.New = func() interface{} {
		return &buffer{}
	}
}

type buffer struct {
	b []byte
	n int
}

func (b *buffer) Grow(n int) {
	m := n - (cap(b.b) - b.n)
	if m > 0 {
		b.b = append(b.b, make([]byte, m)...)
		b.b = b.b[:cap(b.b)]
	}
}

func (b *buffer) Put8(n byte) {
	b.Grow(1)
	b.b[b.n] = n
	b.n++
}

func (b *buffer) Put16(n int) {
	b.Grow(2)
	b.b[b.n] = byte(n >> 8)
	b.n++
	b.b[b.n] = byte(n)
	b.n++
}

func (b *buffer) Put64(n int) {
	b.Grow(8)
	b.b[b.n] = byte(n >> 56)
	b.n++
	b.b[b.n] = byte(n >> 48)
	b.n++
	b.b[b.n] = byte(n >> 40)
	b.n++
	b.b[b.n] = byte(n >> 32)
	b.n++
	b.b[b.n] = byte(n >> 24)
	b.n++
	b.b[b.n] = byte(n >> 16)
	b.n++
	b.b[b.n] = byte(n >> 8)
	b.n++
	b.b[b.n] = byte(n)
	b.n++
}

func (b *buffer) PutRand(n int) {
	b.Grow(n)
	random.Read(b.b[b.n:])
	b.n += n
}

func (b *buffer) PutMask(key, data []byte) {
	n := b.n
	b.Grow(len(data))
	copy(b.b[n:], data)
	for i := 0; i < len(data); i++ {
		b.b[n] ^= key[i%4]
	}
}
