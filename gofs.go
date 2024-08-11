package gofs

import (
	"io"
	"os"
	"time"
)

type FS struct {
	FakeFs *InMemoryFS
}

func OsFs() *FS {
	return nil
}

func (f *FS) Create(name string) (*File, error) {
	if f == nil || f.FakeFs == nil {
		fp, err := os.Create(name)
		return NewFromOs(fp), err
	}
	return f.FakeFs.Create(name)
}

func (f *FS) CreateTemp(dir, pattern string) (*File, error) {
	if f == nil || f.FakeFs == nil {
		fp, err := os.CreateTemp(dir, pattern)
		return NewFromOs(fp), err
	}
	return f.FakeFs.CreateTemp(dir, pattern)
}

func (f *FS) Open(name string) (*File, error) {
	if f == nil || f.FakeFs == nil {
		fp, err := os.Open(name)
		return NewFromOs(fp), err
	}
	return f.FakeFs.Open(name)
}

func (f *FS) OpenFile(name string, flag int, perm os.FileMode) (*File, error) {
	if f == nil || f.FakeFs == nil {
		fp, err := os.OpenFile(name, flag, perm)
		return NewFromOs(fp), err
	}
	return f.FakeFs.OpenFile(name, flag, perm)
}

func (f *FS) Chdir(dir string) error {
	if f == nil || f.FakeFs == nil {
		return os.Chdir(dir)
	}
	return f.FakeFs.Chdir(dir)
}

func (f *FS) Chmod(name string, mode os.FileMode) error {
	if f == nil || f.FakeFs == nil {
		return os.Chmod(name, mode)
	}
	return f.FakeFs.Chmod(name, mode)
}

func (f *FS) Chown(name string, uid, gid int) error {
	if f == nil || f.FakeFs == nil {
		return os.Chown(name, uid, gid)
	}
	return f.FakeFs.Chown(name, uid, gid)
}

func (f *FS) Mkdir(name string, perm os.FileMode) error {
	if f == nil || f.FakeFs == nil {
		return os.Mkdir(name, perm)
	}
	return f.FakeFs.Mkdir(name, perm)
}

func (f *FS) MkdirAll(path string, perm os.FileMode) error {
	if f == nil || f.FakeFs == nil {
		return os.MkdirAll(path, perm)
	}
	return f.FakeFs.MkdirAll(path, perm)
}

func (f *FS) MkdirTemp(dir, pattern string) (string, error) {
	if f == nil || f.FakeFs == nil {
		return os.MkdirTemp(dir, pattern)
	}
	return f.FakeFs.MkdirTemp(dir, pattern)
}

func (f *FS) TempDir() string {
	if f == nil || f.FakeFs == nil {
		return os.TempDir()
	}
	return f.FakeFs.TempDir()
}

func (f *FS) ReadFile(name string) ([]byte, error) {
	if f == nil || f.FakeFs == nil {
		return os.ReadFile(name)
	}
	return f.FakeFs.ReadFile(name)
}

func (f *FS) Readlink(name string) (string, error) {
	if f == nil || f.FakeFs == nil {
		return os.Readlink(name)
	}
	return f.FakeFs.Readlink(name)
}

func (f *FS) Remove(name string) error {
	if f == nil || f.FakeFs == nil {
		return os.Remove(name)
	}
	return f.FakeFs.Remove(name)
}

func (f *FS) RemoveAll(path string) error {
	if f == nil || f.FakeFs == nil {
		return os.RemoveAll(path)
	}
	return f.FakeFs.RemoveAll(path)
}

func (f *FS) Rename(oldpath, newpath string) error {
	if f == nil || f.FakeFs == nil {
		return os.Rename(oldpath, newpath)
	}
	return f.FakeFs.Rename(oldpath, newpath)
}

func (f *FS) Truncate(name string, size int64) error {
	if f == nil || f.FakeFs == nil {
		return os.Truncate(name, size)
	}
	return f.FakeFs.Truncate(name, size)
}

func (f *FS) WriteFile(name string, data []byte, perm os.FileMode) error {
	if f == nil || f.FakeFs == nil {
		return os.WriteFile(name, data, perm)
	}
	return f.FakeFs.WriteFile(name, data, perm)
}

func (f *FS) ReadDir(name string) ([]os.DirEntry, error) {
	if f == nil || f.FakeFs == nil {
		return os.ReadDir(name)
	}
	return f.FakeFs.ReadDir(name)
}

func (f *FS) Stat(name string) (os.FileInfo, error) {
	if f == nil || f.FakeFs == nil {
		return os.Stat(name)
	}
	return f.FakeFs.Stat(name)
}

type File struct {
	mockFile *FakeFile
	osFile   *os.File
}

var _ io.ReadCloser = &File{}
var _ io.WriteCloser = &File{}
var _ io.ReaderAt = &File{}

func NewFromOs(fp *os.File) *File {
	return &File{osFile: fp}
}

func (f *File) Fd() uintptr {
	if f.osFile != nil {
		return f.osFile.Fd()
	}
	return 0
}

func (f *File) Chdir() error {
	if f.osFile != nil {
		return f.osFile.Chdir()
	}
	return f.mockFile.Chdir()
}

func (f *File) Chmod(mode os.FileMode) error {
	if f.osFile != nil {
		return f.osFile.Chmod(mode)
	}
	return f.mockFile.Chmod(mode)
}

func (f *File) Chown(uid, gid int) error {
	if f.osFile != nil {
		return f.osFile.Chown(uid, gid)
	}
	// fs ownership is not implemented
	return nil
}

func (f *File) Close() error {
	if f.osFile != nil {
		return f.osFile.Close()
	}
	return f.mockFile.Close()
}

func (f *File) Name() string {
	if f.osFile != nil {
		return f.osFile.Name()
	}
	return f.mockFile.Name()
}

func (f *File) Read(b []byte) (n int, err error) {
	if f.osFile != nil {
		return f.osFile.Read(b)
	}
	return f.mockFile.Read(b)
}

func (f *File) ReadAt(b []byte, off int64) (n int, err error) {
	if f.osFile != nil {
		return f.osFile.ReadAt(b, off)
	}
	return f.mockFile.ReadAt(b, off)
}

func (f *File) ReadDir(n int) ([]os.DirEntry, error) {
	if f.osFile != nil {
		return f.osFile.ReadDir(n)
	}
	return f.mockFile.ReadDir(n)
}

func (f *File) ReadFrom(r io.Reader) (n int64, err error) {
	if f.osFile != nil {
		return f.osFile.ReadFrom(r)
	}
	return f.mockFile.ReadFrom(r)
}

func (f *File) Readdir(n int) ([]os.FileInfo, error) {
	if f.osFile != nil {
		return f.osFile.Readdir(n)
	}
	return f.mockFile.Readdir(n)
}

func (f *File) Readdirnames(n int) (names []string, err error) {
	if f.osFile != nil {
		return f.osFile.Readdirnames(n)
	}
	return f.mockFile.Readdirnames(n)
}

func (f *File) Seek(offset int64, whence int) (ret int64, err error) {
	if f.osFile != nil {
		return f.osFile.Seek(offset, whence)
	}
	return f.mockFile.Seek(offset, whence)
}

func (f *File) Stat() (os.FileInfo, error) {
	if f.osFile != nil {
		return f.osFile.Stat()
	}
	return f.mockFile.Stat()
}

func (f *File) Sync() error {
	if f.osFile != nil {
		return f.osFile.Sync()
	}
	return f.mockFile.Sync()
}

func (f *File) Truncate(size int64) error {
	if f.osFile != nil {
		return f.osFile.Truncate(size)
	}
	return f.mockFile.Truncate(size)
}

func (f *File) Write(b []byte) (n int, err error) {
	if f.osFile != nil {
		return f.osFile.Write(b)
	}
	return f.mockFile.Write(b)
}

func (f *File) WriteAt(b []byte, off int64) (n int, err error) {
	if f.osFile != nil {
		return f.osFile.WriteAt(b, off)
	}
	return f.mockFile.WriteAt(b, off)
}

func (f *File) WriteString(s string) (n int, err error) {
	if f.osFile != nil {
		return f.osFile.WriteString(s)
	}
	return f.mockFile.WriteString(s)
}

// TODO: support WriteTo for go 1.21?
/*
func (f *File) WriteTo(w io.Writer) (n int64, err error) {
	if f.osFile != nil {
		return f.osFile.WriteTo(w)
	}
	return f.mockFile.WriteTo(w)
}
*/

func (f *File) IsFake() bool {
	return f.osFile == nil
}

func (f *File) SetDeadline(t time.Time) error {
	if f.osFile != nil {
		return f.osFile.SetDeadline(t)
	}
	// noop
	return nil
}

func (f *File) SetReadDeadline(t time.Time) error {
	if f.osFile != nil {
		return f.osFile.SetReadDeadline(t)
	}
	// noop
	return nil
}

func (f *File) SetWriteDeadline(t time.Time) error {
	if f.osFile != nil {
		return f.osFile.SetWriteDeadline(t)
	}
	// noop
	return nil
}
