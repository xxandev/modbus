package modbus

// Longitudinal Redundancy Checking
type lrc struct {
	sum uint8
}

func (lrc *lrc) reset() *lrc {
	lrc.sum = 0
	return lrc
}

func (lrc *lrc) pushByte(b byte) *lrc {
	lrc.sum += b
	return lrc
}

func (lrc *lrc) pushBytes(data []byte) *lrc {
	var b byte
	for _, b = range data {
		lrc.sum += b
	}
	return lrc
}

func (lrc *lrc) value() byte {
	// Return twos complement
	return uint8(-int8(lrc.sum))
}
