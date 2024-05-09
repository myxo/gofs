package util

import "os"

func ResizeSlice(s []byte, size int) []byte {
	if size < 0 {
		panic("negative size")
	}
	if cap(s) < size {
		s = append(s[:cap(s)], make([]byte, size-cap(s))...)
	} else {
		s = s[:size]
	}
	return s
}

func IsWriteOnly(flag int) bool {
	return flag&os.O_WRONLY != 0
}

func IsReadOnly(flag int) bool {
	const mask = 0x3
	return flag&mask == 0 // read obly is an absend of bits
}

func IsReadWrite(flag int) bool {
	return flag&os.O_RDWR != 0
}

func HasWritePerm(flag int) bool {
	return IsWriteOnly(flag) || IsReadWrite(flag)
}

func HasReadPerm(flag int) bool {
	return IsReadOnly(flag) || IsReadWrite(flag)
}

func IsAppend(flag int) bool {
	return flag&os.O_APPEND != 0
}

func IsCreate(flag int) bool {
	return flag&os.O_CREATE != 0
}

func IsExclusive(flag int) bool {
	return flag&os.O_EXCL != 0
}

func IsTruncate(flag int) bool {
	return flag&os.O_TRUNC != 0
}
