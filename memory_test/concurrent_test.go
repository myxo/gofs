package memory

import (
	"io"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/myxo/gofs"
	"github.com/stretchr/testify/require"
)

// gofs does not try to replicate any concurrent behaviour, hence we do not check correctness here.
// We just run this test with -race to see that we can run gofs.NewThreadSafeMemoryFs in concurrent code.
func TestMultithreading(t *testing.T) {
	fs := gofs.NewThreadSafeMemoryFs()
	dir := fs.TempDir()

	fAll, err := fs.Create(filepath.Join(dir, "test"))
	require.NoError(t, err)
	wg := sync.WaitGroup{}

	for i := 0; i < 5; i++ {
		subdir := filepath.Join(dir, "subdir"+strconv.Itoa(i))
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = fAll.Write([]byte("hello world"))
			buff := make([]byte, 100)
			_, _ = fAll.Read(buff)
			_, _ = fAll.Seek(0, io.SeekStart)
			_, _ = fAll.Stat()
			_ = fAll.Sync()

			_ = fs.Chmod(fAll.Name(), 0777)
			_, _ = fs.ReadDir(".")

			_ = fs.Mkdir(subdir, 0777)
			_ = fs.Chdir(subdir)
			f, err := fs.CreateTemp(subdir, "own")
			require.NoError(t, err)
			_, _ = f.Write(buff)

			_ = fs.Chmod(f.Name(), 0777)
			_ = fs.Truncate(f.Name(), 3)
			_, _ = fs.Stat(f.Name())
			_, _ = fs.Stat(subdir)

			_ = fs.Rename(f.Name(), f.Name()+".new")
			_ = fs.Remove(f.Name() + ".new")
			_ = fs.RemoveAll(subdir)
		}()
	}

	wg.Wait()
}
