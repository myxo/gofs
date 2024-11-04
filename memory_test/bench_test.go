package memory

import (
	"crypto/rand"
	"io"
	"os"
	"strconv"
	"testing"

	"github.com/myxo/gofs"
	"github.com/stretchr/testify/require"
)

func BenchmarkInMemory(b *testing.B) {
	fs := gofs.NewMemoryFs()
	fp, err := fs.OpenFile("test", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0666)
	require.NoError(b, err)

	const maxFileSize = 10 * 1024 * 1024

	for _, buffSize := range []int{32, 4 * 1024, 64 * 1024} {
		buff := make([]byte, buffSize)
		_, _ = rand.Read(buff)

		b.Run("write to start "+strconv.Itoa(buffSize), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				n, _ := fp.WriteAt(buff, 0)
				b.SetBytes(int64(n))
			}
		})
		b.Run("read at start "+strconv.Itoa(buffSize), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				n, _ := fp.ReadAt(buff, 0)
				b.SetBytes(int64(n))
			}
		})
		curSize := 0
		b.Run("write cont "+strconv.Itoa(buffSize), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				n, _ := fp.Write(buff)
				b.SetBytes(int64(n))
				curSize += n
				if curSize > maxFileSize {
					_, _ = fp.Seek(0, io.SeekStart)
				}
			}
		})
		curSize = 0
		b.Run("read cont "+strconv.Itoa(buffSize), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				n, _ := fp.Read(buff)
				b.SetBytes(int64(n))
				curSize += n
				if curSize > maxFileSize {
					_, _ = fp.Seek(0, io.SeekStart)
				}
			}
		})
	}

	b.Run("readDir", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = fs.ReadDir("/")
		}
	})

	fpRootDir, err := fs.Open("/")
	require.NoError(b, err)
	b.Run("readDir file", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = fpRootDir.Readdir(-1)
		}
	})
}
