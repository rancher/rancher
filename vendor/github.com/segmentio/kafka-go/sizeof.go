package kafka

import "fmt"

type sizable interface {
	size() int32
}

func sizeof(a interface{}) int32 {
	switch v := a.(type) {
	case int8:
		return 1
	case int16:
		return 2
	case int32:
		return 4
	case int64:
		return 8
	case string:
		return sizeofString(v)
	case bool:
		return 1
	case []byte:
		return sizeofBytes(v)
	case sizable:
		return v.size()
	}
	panic(fmt.Sprintf("unsupported type: %T", a))
}

func sizeofInt8(_ int8) int32 {
	return 1
}

func sizeofInt16(_ int16) int32 {
	return 2
}

func sizeofInt32(_ int32) int32 {
	return 4
}

func sizeofInt64(_ int64) int32 {
	return 8
}

func sizeofString(s string) int32 {
	return 2 + int32(len(s))
}

func sizeofNullableString(s *string) int32 {
	if s == nil {
		return 2
	}
	return sizeofString(*s)
}

func sizeofBool(_ bool) int32 {
	return 1
}

func sizeofBytes(b []byte) int32 {
	return 4 + int32(len(b))
}

func sizeofArray(n int, f func(int) int32) int32 {
	s := int32(4)
	for i := 0; i != n; i++ {
		s += f(i)
	}
	return s
}

func sizeofInt32Array(a []int32) int32 {
	return 4 + (4 * int32(len(a)))
}

func sizeofStringArray(a []string) int32 {
	return sizeofArray(len(a), func(i int) int32 { return sizeofString(a[i]) })
}
