package socket

// 掩码
type Mask struct {
	mask byte    // 是否掩码
	key  [4]byte // key缓存
	idx  int     // 当前数据索引
}

func (m *Mask) ResetIndex() {
	m.idx = 0
}

func (m *Mask) InitKey() []byte {
	_rand.Read(m.key[:])
	return m.key[:]
}

func (m *Mask) Mask(data []byte) {
	for i := 0; i < len(data); i++ {
		data[i] ^= m.key[m.idx%4]
		m.idx++
	}
}
