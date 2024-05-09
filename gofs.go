package gofs

import (
	"io"
	"os"
	"time"
)

type FS interface {
	Create(name string) (*File, error)
	CreateTemp(dir, pattern string) (*File, error)
	// NewFile(fd uintptr, name string) *File // TODO: ???
	Open(name string) (*File, error)
	OpenFile(name string, flag int, perm os.FileMode) (*File, error)
	Chdir(dir string) error
	Chmod(name string, mode os.FileMode) error
	Chown(name string, uid, gid int) error
	Mkdir(name string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	MkdirTemp(dir, pattern string) (string, error)
	ReadFile(name string) ([]byte, error)
	Readlink(name string) (string, error)
	ReadDir(name string) ([]os.DirEntry, error)
	Remove(name string) error
	RemoveAll(path string) error
	Rename(oldpath, newpath string) error
	Truncate(name string, size int64) error
	WriteFile(name string, data []byte, perm os.FileMode) error
	Stat(name string) (os.FileInfo, error)
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
	panic("todo")
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

type osFs struct{}

var _ FS = &osFs{}

func OsFs() FS {
	return &osFs{}
}

func (osFs) Create(name string) (*File, error) {
	fp, err := os.Create(name)
	return NewFromOs(fp), err
}

func (osFs) CreateTemp(dir, pattern string) (*File, error) {
	fp, err := os.CreateTemp(dir, pattern)
	return NewFromOs(fp), err
}

func (osFs) Open(name string) (*File, error) {
	fp, err := os.Open(name)
	return NewFromOs(fp), err
}

func (osFs) OpenFile(name string, flag int, perm os.FileMode) (*File, error) {
	fp, err := os.OpenFile(name, flag, perm)
	return NewFromOs(fp), err
}

func (osFs) Chdir(dir string) error {
	return os.Chdir(dir)
}

func (osFs) Chmod(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
}

func (osFs) Chown(name string, uid, gid int) error {
	return os.Chown(name, uid, gid)
}

func (osFs) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(name, perm)
}

func (osFs) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (osFs) MkdirTemp(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}

func (osFs) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (osFs) Readlink(name string) (string, error) {
	return os.Readlink(name)
}

func (osFs) Remove(name string) error {
	return os.Remove(name)
}

func (osFs) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (osFs) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (osFs) Truncate(name string, size int64) error {
	return os.Truncate(name, size)
}

func (osFs) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (osFs) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}

func (osFs) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}
