package gofs

import "os"

func resizeSlice(s []byte, size int) []byte {
	if size < 0 {
		panic("negative size")
	}
	if size > 1024*1024 {
		panic(size)
	}
	if cap(s) < size {
		s = append(s[:cap(s)], make([]byte, size-cap(s))...)
	} else {
		s = s[:size]
	}
	return s
}

func isWriteOnly(flag int) bool {
	return flag&os.O_WRONLY != 0
}

func isReadOnly(flag int) bool {
	const mask = 0x3
	return flag&mask == 0 // read obly is an absend of bits
}

func isReadWrite(flag int) bool {
	return flag&os.O_RDWR != 0
}

func hasWritePerm(flag int) bool {
	return isWriteOnly(flag) || isReadWrite(flag)
}

func hasReadPerm(flag int) bool {
	return isReadOnly(flag) || isReadWrite(flag)
}

func isAppend(flag int) bool {
	return flag&os.O_APPEND != 0
}

func isCreate(flag int) bool {
	return flag&os.O_CREATE != 0
}

func isExclusive(flag int) bool {
	return flag&os.O_EXCL != 0
}

func isTruncate(flag int) bool {
	return flag&os.O_TRUNC != 0
}
