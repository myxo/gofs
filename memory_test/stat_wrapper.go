package memory

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/myxo/gofs"
)

var stat = map[string]int{}

func addStat(op string) {
	stat[op] = stat[op] + 1
}

func printStat() {
	fmt.Printf("usage statistic (%d operations):\n", len(stat))
	var msg []string
	for k, v := range stat {
		msg = append(msg, fmt.Sprintf("%s:\t%d\n", k, v))
	}
	slices.Sort(msg)
	fmt.Printf("%s", strings.Join(msg, ""))
}

func resetStat() {
	clear(stat)
}

type statFile struct {
	fp *gofs.File
}

func (f *statFile) Fd() uintptr {
	addStat("Fd")
	return f.fp.Fd()
}

func (f *statFile) Chdir() error {
	addStat("Chdir")
	return f.fp.Chdir()
}

func (f *statFile) Chmod(mode os.FileMode) error {
	addStat("Chmod")
	return f.fp.Chmod(mode)
}

func (f *statFile) Chown(uid, gid int) error {
	addStat("Chown")
	return f.fp.Chown(uid, gid)
}

func (f *statFile) Close() error {
	addStat("Close")
	return f.fp.Close()
}

func (f *statFile) Name() string {
	addStat("Name")
	return f.fp.Name()
}

func (f *statFile) Read(b []byte) (n int, err error) {
	addStat("Read")
	return f.fp.Read(b)
}

func (f *statFile) ReadAt(b []byte, off int64) (n int, err error) {
	addStat("ReadAt")
	return f.fp.ReadAt(b, off)
}

func (f *statFile) ReadDir(n int) ([]os.DirEntry, error) {
	addStat("ReadDir")
	return f.fp.ReadDir(n)
}

func (f *statFile) ReadFrom(r io.Reader) (n int64, err error) {
	addStat("ReadFrom")
	return f.fp.ReadFrom(r)
}

func (f *statFile) Readdir(n int) ([]os.FileInfo, error) {
	addStat("Readdir")
	return f.fp.Readdir(n)
}

func (f *statFile) Readdirnames(n int) (names []string, err error) {
	addStat("Readdirnames")
	return f.fp.Readdirnames(n)
}

func (f *statFile) Seek(offset int64, whence int) (ret int64, err error) {
	addStat("Seek")
	return f.fp.Seek(offset, whence)
}

func (f *statFile) Stat() (os.FileInfo, error) {
	addStat("Stat")
	return f.fp.Stat()
}

func (f *statFile) Sync() error {
	addStat("Sync")
	return f.fp.Sync()
}

func (f *statFile) Truncate(size int64) error {
	addStat("Truncate")
	return f.fp.Truncate(size)
}

func (f *statFile) Write(b []byte) (n int, err error) {
	addStat("Write")
	return f.fp.Write(b)
}

func (f *statFile) WriteAt(b []byte, off int64) (n int, err error) {
	addStat("WriteAt")
	return f.fp.WriteAt(b, off)
}

func (f *statFile) WriteString(s string) (n int, err error) {
	addStat("WriteString")
	return f.fp.WriteString(s)
}

/*
func (f *statFile) WriteTo(w io.Writer) (n int64, err error) {
	addStat("WriteTo")
	return f.fp.WriteTo(w)
}
*/

func (f *statFile) IsFake() bool {
	addStat("IsFake")
	return f.fp.IsFake()
}

type statFs struct {
	fs *gofs.InMemoryFS
}

func (s *statFs) Create(name string) (*gofs.File, error) {
	addStat("fs_Create")
	return s.fs.Create(name)
}

func (s *statFs) CreateTemp(dir, pattern string) (*gofs.File, error) {
	addStat("fs_CreateTemp")
	return s.fs.CreateTemp(dir, pattern)
}

func (s *statFs) Open(name string) (*gofs.File, error) {
	addStat("fs_Open")
	return s.fs.Open(name)
}

func (s *statFs) OpenFile(name string, flag int, perm os.FileMode) (*gofs.File, error) {
	addStat("fs_OpenFile")
	return s.fs.OpenFile(name, flag, perm)
}

func (s *statFs) Chdir(dir string) error {
	addStat("fs_Chdir")
	return s.fs.Chdir(dir)
}

func (s *statFs) Chmod(name string, mode os.FileMode) error {
	addStat("fs_Chmod")
	return s.fs.Chmod(name, mode)
}

func (s *statFs) Chown(name string, uid, gid int) error {
	addStat("fs_Chown")
	return s.fs.Chown(name, uid, gid)
}

func (s *statFs) Mkdir(name string, perm os.FileMode) error {
	addStat("fs_Mkdir")
	return s.fs.Mkdir(name, perm)
}

func (s *statFs) MkdirAll(path string, perm os.FileMode) error {
	addStat("fs_MkdirAll")
	return s.fs.MkdirAll(path, perm)
}

func (s *statFs) MkdirTemp(dir, pattern string) (string, error) {
	addStat("fs_MkdirTemp")
	return s.fs.MkdirTemp(dir, pattern)
}

func (s *statFs) ReadFile(name string) ([]byte, error) {
	addStat("fs_ReadFile")
	return s.fs.ReadFile(name)
}

func (s *statFs) Readlink(name string) (string, error) {
	addStat("fs_Readlink")
	return s.fs.Readlink(name)
}

func (s *statFs) Remove(name string) error {
	addStat("fs_Remove")
	return s.fs.Remove(name)
}

func (s *statFs) RemoveAll(path string) error {
	addStat("fs_RemoveAll")
	return s.fs.RemoveAll(path)
}

func (s *statFs) Rename(oldpath, newpath string) error {
	addStat("fs_Rename")
	return s.fs.Rename(oldpath, newpath)
}

func (s *statFs) Truncate(name string, size int64) error {
	addStat("fs_Truncate")
	return s.fs.Truncate(name, size)
}

func (s *statFs) WriteFile(name string, data []byte, perm os.FileMode) error {
	addStat("fs_WriteFile")
	return s.fs.WriteFile(name, data, perm)
}

func (s *statFs) ReadDir(name string) ([]os.DirEntry, error) {
	addStat("fs_ReadDir")
	return s.fs.ReadDir(name)
}

func (s *statFs) Stat(name string) (os.FileInfo, error) {
	addStat("fs_Stat")
	return s.fs.Stat(name)
}
