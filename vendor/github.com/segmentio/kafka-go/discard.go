package kafka

import "bufio"

func discardN(r *bufio.Reader, sz int, n int) (int, error) {
	var err error
	if n <= sz {
		n, err = r.Discard(n)
	} else {
		n, err = r.Discard(sz)
		if err == nil {
			err = errShortRead
		}
	}
	return sz - n, err
}

func discardInt8(r *bufio.Reader, sz int) (int, error) {
	return discardN(r, sz, 1)
}

func discardInt16(r *bufio.Reader, sz int) (int, error) {
	return discardN(r, sz, 2)
}

func discardInt32(r *bufio.Reader, sz int) (int, error) {
	return discardN(r, sz, 4)
}

func discardInt64(r *bufio.Reader, sz int) (int, error) {
	return discardN(r, sz, 8)
}

func discardString(r *bufio.Reader, sz int) (int, error) {
	return readStringWith(r, sz, func(r *bufio.Reader, sz int, n int) (int, error) {
		if n < 0 {
			return sz, nil
		}
		return discardN(r, sz, n)
	})
}

func discardBytes(r *bufio.Reader, sz int) (int, error) {
	return readBytesWith(r, sz, func(r *bufio.Reader, sz int, n int) (int, error) {
		if n < 0 {
			return sz, nil
		}
		return discardN(r, sz, n)
	})
}
