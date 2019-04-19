package kafka

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"reflect"
)

type readable interface {
	readFrom(*bufio.Reader, int) (int, error)
}

var errShortRead = errors.New("not enough bytes available to load the response")

func peekRead(r *bufio.Reader, sz int, n int, f func([]byte)) (int, error) {
	if n > sz {
		return sz, errShortRead
	}
	b, err := r.Peek(n)
	if err != nil {
		return sz, err
	}
	f(b)
	return discardN(r, sz, n)
}

func readInt8(r *bufio.Reader, sz int, v *int8) (int, error) {
	return peekRead(r, sz, 1, func(b []byte) { *v = makeInt8(b) })
}

func readInt16(r *bufio.Reader, sz int, v *int16) (int, error) {
	return peekRead(r, sz, 2, func(b []byte) { *v = makeInt16(b) })
}

func readInt32(r *bufio.Reader, sz int, v *int32) (int, error) {
	return peekRead(r, sz, 4, func(b []byte) { *v = makeInt32(b) })
}

func readInt64(r *bufio.Reader, sz int, v *int64) (int, error) {
	return peekRead(r, sz, 8, func(b []byte) { *v = makeInt64(b) })
}

func readVarInt(r *bufio.Reader, sz int, v *int64) (remain int, err error) {
	l := 0
	remain = sz
	for done := false; !done && err == nil; {
		remain, err = peekRead(r, remain, 1, func(b []byte) {
			done = b[0]&0x80 == 0
			*v |= int64(b[0]&0x7f) << uint(l*7)
		})
		l++
	}
	*v = (*v >> 1) ^ -(*v & 1)
	return
}

func readBool(r *bufio.Reader, sz int, v *bool) (int, error) {
	return peekRead(r, sz, 1, func(b []byte) { *v = b[0] != 0 })
}

func readString(r *bufio.Reader, sz int, v *string) (int, error) {
	return readStringWith(r, sz, func(r *bufio.Reader, sz int, n int) (remain int, err error) {
		*v, remain, err = readNewString(r, sz, n)
		return
	})
}

func readStringWith(r *bufio.Reader, sz int, cb func(*bufio.Reader, int, int) (int, error)) (int, error) {
	var err error
	var len int16

	if sz, err = readInt16(r, sz, &len); err != nil {
		return sz, err
	}

	n := int(len)
	if n > sz {
		return sz, errShortRead
	}

	return cb(r, sz, n)
}

func readNewString(r *bufio.Reader, sz int, n int) (string, int, error) {
	b, sz, err := readNewBytes(r, sz, n)
	return string(b), sz, err
}

func readBytes(r *bufio.Reader, sz int, v *[]byte) (int, error) {
	return readBytesWith(r, sz, func(r *bufio.Reader, sz int, n int) (remain int, err error) {
		*v, remain, err = readNewBytes(r, sz, n)
		return
	})
}

func readBytesWith(r *bufio.Reader, sz int, cb func(*bufio.Reader, int, int) (int, error)) (int, error) {
	var err error
	var n int

	if sz, err = readArrayLen(r, sz, &n); err != nil {
		return sz, err
	}

	if n > sz {
		return sz, errShortRead
	}

	return cb(r, sz, n)
}

func readNewBytes(r *bufio.Reader, sz int, n int) ([]byte, int, error) {
	var err error
	var b []byte
	var shortRead bool

	if n > 0 {
		if sz < n {
			n = sz
			shortRead = true
		}

		b = make([]byte, n)
		n, err = io.ReadFull(r, b)
		b = b[:n]
		sz -= n

		if err == nil && shortRead {
			err = errShortRead
		}
	}

	return b, sz, err
}

func readArrayLen(r *bufio.Reader, sz int, n *int) (int, error) {
	var err error
	var len int32
	if sz, err = readInt32(r, sz, &len); err != nil {
		return sz, err
	}
	*n = int(len)
	return sz, nil
}

func readArrayWith(r *bufio.Reader, sz int, cb func(*bufio.Reader, int) (int, error)) (int, error) {
	var err error
	var len int32

	if sz, err = readInt32(r, sz, &len); err != nil {
		return sz, err
	}

	for n := int(len); n > 0; n-- {
		if sz, err = cb(r, sz); err != nil {
			break
		}
	}

	return sz, err
}

func readStringArray(r *bufio.Reader, sz int, v *[]string) (remain int, err error) {
	var content []string
	fn := func(r *bufio.Reader, size int) (fnRemain int, fnErr error) {
		var value string
		if fnRemain, fnErr = readString(r, size, &value); fnErr != nil {
			return
		}
		content = append(content, value)
		return
	}
	if remain, err = readArrayWith(r, sz, fn); err != nil {
		return
	}

	*v = content
	return
}

func readMapStringInt32(r *bufio.Reader, sz int, v *map[string][]int32) (remain int, err error) {
	var len int32
	if remain, err = readInt32(r, sz, &len); err != nil {
		return
	}

	content := make(map[string][]int32, len)
	for i := 0; i < int(len); i++ {
		var key string
		var values []int32

		if remain, err = readString(r, remain, &key); err != nil {
			return
		}

		fn := func(r *bufio.Reader, size int) (fnRemain int, fnErr error) {
			var value int32
			if fnRemain, fnErr = readInt32(r, size, &value); fnErr != nil {
				return
			}
			values = append(values, value)
			return
		}
		if remain, err = readArrayWith(r, remain, fn); err != nil {
			return
		}

		content[key] = values
	}
	*v = content

	return
}

func read(r *bufio.Reader, sz int, a interface{}) (int, error) {
	switch v := a.(type) {
	case *int8:
		return readInt8(r, sz, v)
	case *int16:
		return readInt16(r, sz, v)
	case *int32:
		return readInt32(r, sz, v)
	case *int64:
		return readInt64(r, sz, v)
	case *bool:
		return readBool(r, sz, v)
	case *string:
		return readString(r, sz, v)
	case *[]byte:
		return readBytes(r, sz, v)
	}
	switch v := reflect.ValueOf(a).Elem(); v.Kind() {
	case reflect.Struct:
		return readStruct(r, sz, v)
	case reflect.Slice:
		return readSlice(r, sz, v)
	default:
		panic(fmt.Sprintf("unsupported type: %T", a))
	}
}

func readAll(r *bufio.Reader, sz int, ptrs ...interface{}) (int, error) {
	var err error

	for _, ptr := range ptrs {
		if sz, err = readPtr(r, sz, ptr); err != nil {
			break
		}
	}

	return sz, err
}

func readPtr(r *bufio.Reader, sz int, ptr interface{}) (int, error) {
	switch v := ptr.(type) {
	case *int8:
		return readInt8(r, sz, v)
	case *int16:
		return readInt16(r, sz, v)
	case *int32:
		return readInt32(r, sz, v)
	case *int64:
		return readInt64(r, sz, v)
	case *string:
		return readString(r, sz, v)
	case *[]byte:
		return readBytes(r, sz, v)
	case readable:
		return v.readFrom(r, sz)
	default:
		panic(fmt.Sprintf("unsupported type: %T", v))
	}
}

func readStruct(r *bufio.Reader, sz int, v reflect.Value) (int, error) {
	var err error
	for i, n := 0, v.NumField(); i != n; i++ {
		if sz, err = read(r, sz, v.Field(i).Addr().Interface()); err != nil {
			return sz, err
		}
	}
	return sz, nil
}

func readSlice(r *bufio.Reader, sz int, v reflect.Value) (int, error) {
	var err error
	var len int32

	if sz, err = readInt32(r, sz, &len); err != nil {
		return sz, err
	}

	if n := int(len); n < 0 {
		v.Set(reflect.Zero(v.Type()))
	} else {
		v.Set(reflect.MakeSlice(v.Type(), n, n))

		for i := 0; i != n; i++ {
			if sz, err = read(r, sz, v.Index(i).Addr().Interface()); err != nil {
				return sz, err
			}
		}
	}

	return sz, nil
}

func readFetchResponseHeaderV2(r *bufio.Reader, size int) (throttle int32, watermark int64, remain int, err error) {
	var n int32
	var p struct {
		Partition           int32
		ErrorCode           int16
		HighwaterMarkOffset int64
		MessageSetSize      int32
	}

	if remain, err = readInt32(r, size, &throttle); err != nil {
		return
	}

	if remain, err = readInt32(r, remain, &n); err != nil {
		return
	}

	// This error should never trigger, unless there's a bug in the kafka client
	// or server.
	if n != 1 {
		err = fmt.Errorf("1 kafka topic was expected in the fetch response but the client received %d", n)
		return
	}

	// We ignore the topic name because we've requests messages for a single
	// topic, unless there's a bug in the kafka server we will have received
	// the name of the topic that we requested.
	if remain, err = discardString(r, remain); err != nil {
		return
	}

	if remain, err = readInt32(r, remain, &n); err != nil {
		return
	}

	// This error should never trigger, unless there's a bug in the kafka client
	// or server.
	if n != 1 {
		err = fmt.Errorf("1 kafka partition was expected in the fetch response but the client received %d", n)
		return
	}

	if remain, err = read(r, remain, &p); err != nil {
		return
	}

	if p.ErrorCode != 0 {
		err = Error(p.ErrorCode)
		return
	}

	// This error should never trigger, unless there's a bug in the kafka client
	// or server.
	if remain != int(p.MessageSetSize) {
		err = fmt.Errorf("the size of the message set in a fetch response doesn't match the number of remaining bytes (message set size = %d, remaining bytes = %d)", p.MessageSetSize, remain)
		return
	}

	watermark = p.HighwaterMarkOffset
	return
}

func readFetchResponseHeaderV5(r *bufio.Reader, size int) (throttle int32, watermark int64, remain int, err error) {
	var n int32
	type AbortedTransaction struct {
		ProducerId  int64
		FirstOffset int64
	}
	var p struct {
		Partition           int32
		ErrorCode           int16
		HighwaterMarkOffset int64
		LastStableOffset    int64
		LogStartOffset      int64
	}
	var messageSetSize int32
	var abortedTransactions []AbortedTransaction

	if remain, err = readInt32(r, size, &throttle); err != nil {
		return
	}

	if remain, err = readInt32(r, remain, &n); err != nil {
		return
	}

	// This error should never trigger, unless there's a bug in the kafka client
	// or server.
	if n != 1 {
		err = fmt.Errorf("1 kafka topic was expected in the fetch response but the client received %d", n)
		return
	}

	// We ignore the topic name because we've requests messages for a single
	// topic, unless there's a bug in the kafka server we will have received
	// the name of the topic that we requested.
	if remain, err = discardString(r, remain); err != nil {
		return
	}

	if remain, err = readInt32(r, remain, &n); err != nil {
		return
	}

	// This error should never trigger, unless there's a bug in the kafka client
	// or server.
	if n != 1 {
		err = fmt.Errorf("1 kafka partition was expected in the fetch response but the client received %d", n)
		return
	}

	if remain, err = read(r, remain, &p); err != nil {
		return
	}

	var abortedTransactionLen int
	if remain, err = readArrayLen(r, remain, &abortedTransactionLen); err != nil {
		return
	}

	if abortedTransactionLen == -1 {
		abortedTransactions = nil
	} else {
		abortedTransactions = make([]AbortedTransaction, abortedTransactionLen)
		for i := 0; i < abortedTransactionLen; i++ {
			if remain, err = read(r, remain, &abortedTransactions[i]); err != nil {
				return
			}
		}
	}

	if p.ErrorCode != 0 {
		err = Error(p.ErrorCode)
		return
	}

	remain, err = readInt32(r, remain, &messageSetSize)
	if err != nil {
		return
	}

	// This error should never trigger, unless there's a bug in the kafka client
	// or server.
	if remain != int(messageSetSize) {
		err = fmt.Errorf("the size of the message set in a fetch response doesn't match the number of remaining bytes (message set size = %d, remaining bytes = %d)", messageSetSize, remain)
		return
	}

	watermark = p.HighwaterMarkOffset
	return

}

func readFetchResponseHeaderV10(r *bufio.Reader, size int) (throttle int32, watermark int64, remain int, err error) {
	var n int32
	var errorCode int16
	type AbortedTransaction struct {
		ProducerId  int64
		FirstOffset int64
	}
	var p struct {
		Partition           int32
		ErrorCode           int16
		HighwaterMarkOffset int64
		LastStableOffset    int64
		LogStartOffset      int64
	}
	var messageSetSize int32
	var abortedTransactions []AbortedTransaction

	if remain, err = readInt32(r, size, &throttle); err != nil {
		return
	}

	if remain, err = readInt16(r, remain, &errorCode); err != nil {
		return
	}
	if errorCode != 0 {
		err = Error(errorCode)
		return
	}

	if remain, err = discardInt32(r, remain); err != nil {
		return
	}

	if remain, err = readInt32(r, remain, &n); err != nil {
		return
	}

	// This error should never trigger, unless there's a bug in the kafka client
	// or server.
	if n != 1 {
		err = fmt.Errorf("1 kafka topic was expected in the fetch response but the client received %d", n)
		return
	}

	// We ignore the topic name because we've requests messages for a single
	// topic, unless there's a bug in the kafka server we will have received
	// the name of the topic that we requested.
	if remain, err = discardString(r, remain); err != nil {
		return
	}

	if remain, err = readInt32(r, remain, &n); err != nil {
		return
	}

	// This error should never trigger, unless there's a bug in the kafka client
	// or server.
	if n != 1 {
		err = fmt.Errorf("1 kafka partition was expected in the fetch response but the client received %d", n)
		return
	}

	if remain, err = read(r, remain, &p); err != nil {
		return
	}

	var abortedTransactionLen int
	if remain, err = readArrayLen(r, remain, &abortedTransactionLen); err != nil {
		return
	}

	if abortedTransactionLen == -1 {
		abortedTransactions = nil
	} else {
		abortedTransactions = make([]AbortedTransaction, abortedTransactionLen)
		for i := 0; i < abortedTransactionLen; i++ {
			if remain, err = read(r, remain, &abortedTransactions[i]); err != nil {
				return
			}
		}
	}

	if p.ErrorCode != 0 {
		err = Error(p.ErrorCode)
		return
	}

	remain, err = readInt32(r, remain, &messageSetSize)
	if err != nil {
		return
	}

	// This error should never trigger, unless there's a bug in the kafka client
	// or server.
	if remain != int(messageSetSize) {
		err = fmt.Errorf("the size of the message set in a fetch response doesn't match the number of remaining bytes (message set size = %d, remaining bytes = %d)", messageSetSize, remain)
		return
	}

	watermark = p.HighwaterMarkOffset
	return

}

func readMessageHeader(r *bufio.Reader, sz int) (offset int64, attributes int8, timestamp int64, remain int, err error) {
	var version int8

	if remain, err = readInt64(r, sz, &offset); err != nil {
		return
	}

	// On discarding the message size and CRC:
	// ---------------------------------------
	//
	// - Not sure why kafka gives the message size here, we already have the
	// number of remaining bytes in the response and kafka should only truncate
	// the trailing message.
	//
	// - TCP is already taking care of ensuring data integrity, no need to
	// waste resources doing it a second time so we just skip the message CRC.
	//
	if remain, err = discardN(r, remain, 8); err != nil {
		return
	}

	if remain, err = readInt8(r, remain, &version); err != nil {
		return
	}

	if remain, err = readInt8(r, remain, &attributes); err != nil {
		return
	}

	switch version {
	case 0:
	case 1:
		remain, err = readInt64(r, remain, &timestamp)
	default:
		err = fmt.Errorf("unsupported message version %d found in fetch response", version)
	}

	return
}
