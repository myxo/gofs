package gofs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"github.com/myxo/gofs/internal/util"
)

func MakeError(op string, path string, text string) error {
	return &os.PathError{
		Op:   op,
		Path: path,
		Err:  errors.New(text),
	}
}

func MakeWrappedError(op string, path string, err error) error {
	if err == nil || err == io.EOF {
		return err
	}
	return &os.PathError{
		Op:   op,
		Path: path,
		Err:  err,
	}
}

var filePool = sync.Pool{New: func() any {
	return &mockData{
		buff: make([]byte, 0, 32*1024),
	}
}}

type interval struct {
	from, to int64
}

type mockData struct {
	buff        []byte
	realName    string // this is synced with inodes map
	isDirectory bool
	fs          *FakeFS // TODO: move to FakeFile?
	parent      *mockData
	perm        os.FileMode
	dyrtyPages  []interval // well... it's not exactly pages...
}

func (m *mockData) reset() {
	clear(m.buff[:cap(m.buff)]) // Zero all elements
	m.buff = m.buff[:0]
	m.perm = 0
	m.isDirectory = false
}

func (m *mockData) Size() int64 {
	return int64(len(m.buff))
}

func (m *mockData) hasWritePerm() bool {
	const anyWritePerm = 0222
	return m.perm&anyWritePerm != 0
}

func (m *mockData) hasReadPerm() bool {
	const anyReadPerm = 0444
	return m.perm&anyReadPerm != 0
}

// Kinda like descriptor
type FakeFile struct {
	data          *mockData
	name          string
	flag          int
	cursor        int64
	readDirSlice  []os.DirEntry // non empty only on directory iteration with ReadDir function
	readDirSlice2 []os.FileInfo // non empty only on directory iteration with ReadDir function
}

func (f *FakeFile) Chdir() error {
	if f.data == nil {
		return os.ErrInvalid
	}
	if !f.data.isDirectory {
		return MakeError("Chdir", f.name, "not a directory")
	}
	return f.data.fs.Chdir(f.name)
}

func (f *FakeFile) Chmod(mode os.FileMode) error {
	if f.data == nil {
		return os.ErrInvalid
	}
	f.data.perm = mode & fs.ModePerm
	return nil
}

func (f *FakeFile) Chown(uid, gid int) error { panic("todo") }

func (f *FakeFile) Close() error {
	if f.data == nil {
		return os.ErrInvalid
	}
	if err := f.Sync(); err != nil {
		// TODO: should I release memory here?
		return err
	}
	// cannot reset all variables, since go implementation does not do it
	f.data = nil
	clear(f.readDirSlice)
	clear(f.readDirSlice2)
	return nil
}

func (f *FakeFile) Name() string {
	return f.name
}

func (f *FakeFile) Read(b []byte) (n int, err error) {
	n, err = f.pread(b, f.cursor)
	f.cursor += int64(n)
	return n, MakeWrappedError("Read", f.name, err)
}

func (f *FakeFile) ReadAt(b []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, MakeError("ReadAt", f.name, "negative offset")
	}
	// Mimic weird implementation of ReadAt from stdlib
	for len(b) > 0 {
		m, e := f.pread(b, off)
		if e != nil {
			err = e
			break
		}
		n += m
		b = b[m:]
		off += int64(m)
	}
	return n, MakeWrappedError("ReadAt", f.name, err)
}

func (f *FakeFile) pread(b []byte, off int64) (n int, err error) {
	if f.data == nil {
		return 0, os.ErrInvalid
	}
	if off < 0 {
		return 0, fmt.Errorf("negative offset")
	}
	if len(b) == 0 {
		return 0, nil
	}
	if !util.HasReadPerm(f.flag) {
		return 0, fmt.Errorf("%w file open without write permission", os.ErrPermission)
	}
	if off > int64(len(f.data.buff)) {
		return 0, io.EOF
	}
	n = copy(b, f.data.buff[off:])
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (f *FakeFile) ReadDir(n int) ([]os.DirEntry, error) {
	if f.data == nil {
		return nil, os.ErrInvalid
	}
	if !f.data.isDirectory {
		return nil, MakeError("ReadDir", f.name, "not a directory")
	}
	if f.readDirSlice == nil {
		content, err := f.data.fs.getDirContent(f.name)
		_ = err // TODO
		for i := range content {
			f.readDirSlice = append(f.readDirSlice, NewInfoDataFromNode(content[i], content[i].realName))
		}
	}
	if n > 0 {
		n = min(n, len(f.readDirSlice))
	} else {
		n = len(f.readDirSlice)
	}
	ret := f.readDirSlice[:n]
	f.readDirSlice = f.readDirSlice[n:]
	if len(f.readDirSlice) == 0 {
		f.readDirSlice = nil
	}
	return ret, nil
}

func (f *FakeFile) Readdir(n int) ([]os.FileInfo, error) {
	if f.data == nil {
		return nil, os.ErrInvalid
	}
	if !f.data.isDirectory {
		return nil, MakeError("ReadDir", f.name, "not a directory")
	}
	if f.readDirSlice2 == nil {
		content, err := f.data.fs.getDirContent(f.name)
		_ = err // TODO
		for i := range content {
			f.readDirSlice2 = append(f.readDirSlice2, NewInfoDataFromNode(content[i], content[i].realName))
		}
	}
	if n > 0 {
		n = min(n, len(f.readDirSlice2))
	} else {
		n = len(f.readDirSlice2)
	}
	ret := f.readDirSlice2[:n]
	f.readDirSlice2 = f.readDirSlice2[n:]
	if len(f.readDirSlice2) == 0 {
		f.readDirSlice2 = nil
	}
	return ret, nil
}

func (f *FakeFile) Readdirnames(n int) (names []string, err error) {
	di, err := f.ReadDir(n)
	out := make([]string, len(di))
	for i := range di {
		out[i] = di[i].Name()
	}
	return out, err
}

func (f *FakeFile) ReadFrom(r io.Reader) (n int64, err error) {
	return io.Copy(fileWithoutReadFrom{FakeFile: f}, r)
}

// Hack copypasted from stdlib
// noReadFrom can be embedded alongside another type to
// hide the ReadFrom method of that other type.
//
//nolint:all
type noReadFrom struct{}

// ReadFrom hides another ReadFrom method.
// It should never be called.
//
//nolint:all
func (noReadFrom) ReadFrom(io.Reader) (int64, error) {
	panic("can't happen")
}

// fileWithoutReadFrom implements all the methods of *File other
// than ReadFrom. This is used to permit ReadFrom to call io.Copy
// without leading to a recursive call to ReadFrom.
type fileWithoutReadFrom struct {
	//nolint:all
	noReadFrom
	*FakeFile
}

func (f *FakeFile) Seek(offset int64, whence int) (ret int64, err error) {
	if f.data == nil {
		return 0, os.ErrInvalid
	}
	newOffset := int64(0)
	start := int64(0)
	switch whence {
	case io.SeekStart:
		start = 0
	case io.SeekCurrent:
		start = f.cursor
	case io.SeekEnd:
		start = f.data.Size()
	}
	newOffset = start + offset
	if newOffset < 0 {
		return 0, MakeError("Seek", f.name, "seek offset is negative")
	}
	f.cursor = newOffset
	return newOffset, nil
}

func (f *FakeFile) Stat() (os.FileInfo, error) {
	if f.data == nil {
		return nil, os.ErrInvalid
	}
	// TODO: check read persmissions?
	info := NewInfoDataFromNode(f.data, f.name)
	return info, nil
}

func (f *FakeFile) Sync() error {
	if f.data == nil {
		return os.ErrInvalid
	}
	f.data.dyrtyPages = f.data.dyrtyPages[:0]
	return nil
}

func (f *FakeFile) Truncate(size int64) error {
	if f.data == nil {
		return os.ErrInvalid
	}
	if size < 0 {
		return MakeError("Truncate", f.name, "negative truncate size")
	}
	if !util.HasWritePerm(f.flag) {
		return MakeWrappedError("Truncate", f.name, os.ErrInvalid) // yes, not ErrPermission
	}
	f.data.buff = util.ResizeSlice(f.data.buff, int(size))
	clear(f.data.buff[len(f.data.buff):cap(f.data.buff)])
	return nil
}

func (f *FakeFile) Write(b []byte) (n int, err error) {
	if f.data == nil {
		return 0, os.ErrInvalid
	}
	writePos := f.cursor
	if util.IsAppend(f.flag) {
		writePos = f.data.Size()
	}
	n, err = f.pwrite(b, writePos)
	// what with cursor with append flag? It doesn't matter?
	f.cursor = writePos + int64(n)
	return n, MakeWrappedError("Write", f.name, err)
}

func (f *FakeFile) WriteAt(b []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, fmt.Errorf("negative offset")
	}
	if util.IsAppend(f.flag) {
		return 0, fmt.Errorf("invalid use of WriteAt on file opened with O_APPEND")
	}

	// Mimic weird WriteAt implementation of stdlib
	for len(b) > 0 {
		m, e := f.pwrite(b, off)
		if e != nil {
			err = e
			break
		}
		n += m
		b = b[m:]
		off += int64(m)
	}
	return n, MakeWrappedError("WriteAt", f.name, err)
}

func (f *FakeFile) pwrite(b []byte, off int64) (n int, err error) {
	if f.data == nil {
		return 0, os.ErrInvalid
	}
	if off < 0 {
		return 0, fmt.Errorf("negative offset")
	}

	if !util.IsReadWrite(f.flag) && !util.IsWriteOnly(f.flag) {
		return 0, fmt.Errorf("%w file open wiithout write permission", os.ErrPermission)
	}

	if len(b) == 0 {
		return 0, nil
	}

	if len(f.data.buff) < int(off)+len(b) {
		f.data.buff = util.ResizeSlice(f.data.buff, int(off)+len(b))
	}
	n = copy(f.data.buff[off:], b)

	f.appendDirtyPage(off, off+int64(n))
	return n, nil
}

func (f *FakeFile) appendDirtyPage(from int64, to int64) {
	if !f.data.fs.trackDirtyPages {
		return
	}

	// TODO: want to add some optimization to less allocation (e.g. for situation then we write sequentially)
	f.data.dyrtyPages = append(f.data.dyrtyPages, interval{from: from, to: to})
}

func (f *FakeFile) WriteString(s string) (n int, err error) {
	b := unsafe.Slice(unsafe.StringData(s), len(s))
	return f.Write(b)
}

/*
// noWriteTo can be embedded alongside another type to
// hide the WriteTo method of that other type.
type noWriteTo struct{}

// WriteTo hides another WriteTo method.
// It should never be called.
func (noWriteTo) WriteTo(io.Writer) (int64, error) {
	panic("can't happen")
}

// fileWithoutWriteTo implements all the methods of *File other
// than WriteTo. This is used to permit WriteTo to call io.Copy
// without leading to a recursive call to WriteTo.
type fileWithoutWriteTo struct {
	noWriteTo
	*FakeFile
}

func (f *FakeFile) WriteTo(w io.Writer) (n int64, err error) {
	// TODO: can I just copy, without fileWithoutWriteTo?
	return io.Copy(w, fileWithoutWriteTo{FakeFile: f})
}
*/

type infoData struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool // TODO: feels like it may be in mode
}

var _ os.FileInfo = &infoData{}
var _ os.DirEntry = &infoData{}

func NewInfoDataFromNode(inode *mockData, name string) *infoData {
	// TODO: think how we can use sync.Pool here
	var info infoData
	info.name = filepath.Base(name)
	info.size = inode.Size()
	info.mode = inode.perm
	info.isDir = inode.isDirectory
	return &info
}

func (m *infoData) Name() string {
	return filepath.Base(m.name)
}

func (m *infoData) Size() int64 {
	return m.size
}

func (m *infoData) Mode() os.FileMode {
	return m.mode
}

func (m *infoData) ModTime() time.Time {
	// TODO: make possibility to change modTime for test (e.g. via special function)
	return m.modTime
}

func (m *infoData) IsDir() bool {
	return m.isDir
}

func (m *infoData) Sys() any {
	return nil
}

func (m *infoData) Type() os.FileMode {
	return m.mode
}

func (m *infoData) Info() (os.FileInfo, error) {
	return m, nil
}
