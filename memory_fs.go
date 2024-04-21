package gofs

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
)

type FakeFS struct {
	inodes map[string]*mockData
}

var _ FS = &FakeFS{}

const rootDir = "/"

func NewMemoryFs() *FakeFS {
	fs := &FakeFS{
		inodes: map[string]*mockData{},
	}
	fs.inodes[rootDir] = &mockData{
		realName:    rootDir,
		isDirectory: true,
		perm:        0666,
	}
	return fs
}

func (f *FakeFS) Create(path string) (*File, error) {
	return f.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

func (f *FakeFS) CreateTemp(dir, pattern string) (*File, error) {
	panic("todo")
}

func (f *FakeFS) Open(name string) (*File, error) {
	return f.OpenFile(name, os.O_RDONLY, 0)
}

func checkOpenPerm(flag int, inode *mockData) error {
	if hasWritePerm(flag) && !inode.hasWritePerm() {
		return fmt.Errorf("file does not have write perm")
	}
	if hasReadPerm(flag) && !inode.hasReadPerm() {
		return fmt.Errorf("file does not have read perm")
	}
	return nil
}

func (f *FakeFS) OpenFile(name string, flag int, perm os.FileMode) (*File, error) {
	dirPath := filepath.Dir(name)
	dir, dirExist := f.inodes[dirPath]
	if !dirExist || !dir.isDirectory {
		return nil, fmt.Errorf("dir not exist")
	}
	inode, ok := f.inodes[name]
	if !ok {
		if !isCreate(flag) {
			return nil, fmt.Errorf("no such file %q", name)
		}
		// TODO: check directory perms
		inode = filePool.Get().(*mockData)
		inode.reset()
		inode.realName = name
		inode.perm = perm
		if !isCreate(flag) { // read and write allowed with any perm if you just created the file
			if err := checkOpenPerm(flag, inode); err != nil {
				return nil, err
			}
		}
		f.inodes[name] = inode
		dir.dirContent = append(dir.dirContent, inode)
	} else {
		if err := checkOpenPerm(flag, inode); err != nil {
			return nil, err
		}
		if isCreate(flag) && isExclusive(flag) {
			return nil, fmt.Errorf("file already exist")
		}
		if isTruncate(flag) {
			if !inode.hasWritePerm() {
				return nil, fmt.Errorf("file does not have write perm")
			}
			clear(inode.buff)
			inode.buff = inode.buff[:0]
		}
	}

	return &File{
		mockFile: &FakeFile{
			name: name,
			data: inode,
			flag: flag,
			fs:   f,
		},
	}, nil
}

func (f *FakeFS) Chdir(dir string) error                    { panic("TODO") }
func (f *FakeFS) Chmod(name string, mode os.FileMode) error { panic("TODO") }
func (f *FakeFS) Chown(name string, uid, gid int) error     { panic("TODO") }

func (f *FakeFS) Mkdir(name string, perm os.FileMode) error {
	parentPath := filepath.Dir(name)
	parent, parentExist := f.inodes[parentPath]
	if !parentExist || !parent.isDirectory {
		return fmt.Errorf("parent %q not exist", parentPath)
	}

	if _, exist := f.inodes[name]; exist {
		return fmt.Errorf("already exist")
	}

	inode := &mockData{
		realName:    name,
		isDirectory: true,
		perm:        perm,
	}
	f.inodes[name] = inode
	parent.dirContent = append(parent.dirContent, inode)
	return nil
}

func (f *FakeFS) MkdirAll(path string, perm os.FileMode) error  { panic("TODO") }
func (f *FakeFS) MkdirTemp(dir, pattern string) (string, error) { panic("TODO") }

func (f *FakeFS) ReadFile(name string) ([]byte, error) {
	fp, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	return io.ReadAll(fp)
}

func (f *FakeFS) ReadDir(name string) ([]os.DirEntry, error) {
	panic("todo")
}

func (f *FakeFS) Readlink(name string) (string, error) { panic("TODO") }
func (f *FakeFS) Remove(name string) error             { panic("TODO") }
func (f *FakeFS) RemoveAll(path string) error          { panic("TODO") }

func (f *FakeFS) Rename(oldpath, newpath string) error {
	inode, ok := f.inodes[oldpath]
	if !ok {
		return fmt.Errorf("file not exist (%s)", oldpath)
	}
	dir := filepath.Dir(newpath)
	dirNode, ok := f.inodes[dir]
	if !ok {
		return fmt.Errorf("directory on new path does not exis (%s)", newpath)
	}

	if !dirNode.isDirectory {
		return fmt.Errorf("%s is not a directory", dir)
	}

	delete(f.inodes, oldpath)
	f.inodes[newpath] = inode
	inode.realName = newpath
	return nil
}

func (f *FakeFS) Truncate(name string, size int64) error {
	inode, ok := f.inodes[name]
	if !ok {
		return fmt.Errorf("file not exist")
	}
	if !inode.hasWritePerm() {
		return fmt.Errorf("file does not have write perm")
	}
	// TODO: check write permission
	inode.buff = resizeSlice(inode.buff, int(size))
	clear(inode.buff[len(inode.buff):cap(inode.buff)])
	return nil
}

func (f *FakeFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	fp, err := f.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, err = fp.Write(data)
	if err1 := fp.Close(); err1 != nil && err == nil {
		err = err1
	}
	return err
}

// Release return all memory to pool and clear file map. All File structs should be destroyed by this moment,
// accessing them is UB. This function is useful, for example in end of a test
func (f *FakeFS) Release() {
	for _, v := range f.inodes {
		if v != nil {
			filePool.Put(v)
		}
	}
	clear(f.inodes)
}

func (f *FakeFS) Stat(name string) (os.FileInfo, error) {
	inode, ok := f.inodes[name]
	if !ok {
		return nil, fmt.Errorf("file not exist")
	}
	// TODO: check read persmissions?
	info := NewInfoDataFromNode(inode, inode.realName)
	return info, nil
}

// This function will probably changed by v1.0
func (f *FakeFS) CorruptFile(path string, offset int64) error {
	fp, ok := f.inodes[path]
	if !ok {
		return fmt.Errorf("cannot find file")
	}
	if offset < 0 || offset >= fp.Size() {
		return fmt.Errorf("offset is out of file")
	}
	fp.buff[offset]++
	return nil
}

// This function will probably changed by v1.0
func (f *FakeFS) CorruptDirtyPages(seedRand *rand.Rand) {
	for _, data := range f.inodes {
		for _, dirtyInterval := range data.dyrtyPages {
			flipByte := seedRand.Int63n(dirtyInterval.to - dirtyInterval.from) + dirtyInterval.from
			if flipByte < int64(len(data.buff)) { // TODO: do I need this if?
				data.buff[flipByte]++
			}
		}
	}
}
