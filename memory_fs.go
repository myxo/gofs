package gofs

import (
	"cmp"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"slices"

	"github.com/myxo/gofs/internal/util"
)

type FakeFS struct {
	inodes  map[string]*mockData
	workDir string
}

var _ FS = &FakeFS{}

const rootDir = "/"

func NewMemoryFs() *FakeFS {
	fs := &FakeFS{
		inodes:  map[string]*mockData{},
		workDir: rootDir,
	}
	fs.inodes[rootDir] = &mockData{
		realName:    rootDir,
		isDirectory: true,
		perm:        0666,
		fs:          fs,
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
	if util.HasWritePerm(flag) && !inode.hasWritePerm() {
		return os.ErrPermission
	}
	if util.HasReadPerm(flag) && !inode.hasReadPerm() {
		return os.ErrPermission
	}
	return nil
}

func (f *FakeFS) normilizePath(path string) string {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(f.workDir, path)
	}
	return path
}

func (f *FakeFS) OpenFile(name string, flag int, perm os.FileMode) (*File, error) {
	name = f.normilizePath(name)
	dirPath := filepath.Dir(name)
	dir, dirExist := f.inodes[dirPath]
	if !dirExist || !dir.isDirectory {
		return nil, MakeWrappedError("OpenFile", name, os.ErrNotExist)
	}
	inode, ok := f.inodes[name]
	if !ok {
		if !util.IsCreate(flag) {
			return nil, MakeWrappedError("OpenFile", name, os.ErrNotExist)
		}
		// TODO: check directory perms
		inode = filePool.Get().(*mockData)
		inode.reset()
		inode.realName = name
		inode.perm = perm
		inode.fs = f
		inode.parent = dir
		if !util.IsCreate(flag) { // read and write allowed with any perm if you just created the file
			if err := checkOpenPerm(flag, inode); err != nil {
				return nil, MakeWrappedError("OpenFile", name, os.ErrNotExist)
			}
		}
		f.inodes[name] = inode
	} else {
		if util.IsCreate(flag) && util.IsExclusive(flag) {
			return nil, MakeWrappedError("OpenFile", name, os.ErrExist)
		}
		if err := checkOpenPerm(flag, inode); err != nil {
			return nil, MakeWrappedError("OpenFile", name, err)
		}
		if util.IsTruncate(flag) {
			if !inode.hasWritePerm() {
				return nil, MakeWrappedError("OpenFile", name, os.ErrPermission)
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
		},
	}, nil
}

func (f *FakeFS) Chdir(dir string) error {
	dir = f.normilizePath(dir)
	inode, ok := f.inodes[dir]
	if !ok {
		return MakeWrappedError("Chdir", dir, os.ErrNotExist)
	}
	if !inode.isDirectory {
		return MakeError("Chdir", dir, "not an directory")
	}

	f.workDir = dir
	return nil
}

func (f *FakeFS) Chmod(name string, mode os.FileMode) error {
	name = f.normilizePath(name)
	inode, ok := f.inodes[name]
	if !ok {
		return MakeWrappedError("Chmod", name, os.ErrNotExist)
	}
	inode.perm = mode & fs.ModePerm
	return nil
}

func (f *FakeFS) Chown(name string, uid, gid int) error { panic("TODO") }

func (f *FakeFS) Mkdir(name string, perm os.FileMode) error {
	name = f.normilizePath(name)
	parentPath := filepath.Dir(name)
	parent, parentExist := f.inodes[parentPath]
	if !parentExist || !parent.isDirectory {
		return MakeError("Mkdir", name, "parent path does not exist")
	}

	if _, exist := f.inodes[name]; exist {
		return MakeWrappedError("Mkdir", name, os.ErrExist)
	}

	inode := &mockData{
		realName:    name,
		isDirectory: true,
		perm:        perm,
		fs:          f,
		parent:      parent,
	}
	f.inodes[name] = inode
	return nil
}

func (f *FakeFS) MkdirAll(path string, perm os.FileMode) error {
	parentPath := filepath.Dir(path)
	if parentPath == "." {
		// TODO: check if we catch this if in test
		parentPath = f.workDir
	}
	parent, parentExist := f.inodes[parentPath]
	if !parentExist {
		if err := f.MkdirAll(parentPath, perm); err != nil {
			return err
		}
		parent, parentExist = f.inodes[parentPath]
		if !parentExist {
			panic("internal error: cannot create parent directory")
		}
	} else if !parent.isDirectory {
		return MakeError("MkdirAll", path, "parent path exist, but is't not a directory")
	}

	if _, exist := f.inodes[path]; exist {
		return nil
	}

	inode := &mockData{
		realName:    path,
		isDirectory: true,
		perm:        perm,
		fs:          f,
		parent:      parent,
	}
	f.inodes[path] = inode
	return nil
}

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
	fp, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	dirs, err := fp.ReadDir(-1)
	slices.SortFunc(dirs, func (a os.DirEntry, b os.DirEntry) int { return cmp.Compare(a.Name(), b.Name())})
	return dirs, err
}

func (f *FakeFS) Readlink(name string) (string, error) { panic("TODO") }

func (f *FakeFS) Remove(name string) error {
	return f.remove(name, false)
}

func (f *FakeFS) RemoveAll(path string) error {
	return f.remove(path, true)
}

func (f *FakeFS) remove(name string, all bool) error {
	name = f.normilizePath(name)
	inode, ok := f.inodes[name]
	if !ok {
		if all {
			return nil
		}
		return MakeWrappedError("Remove", name, os.ErrNotExist)
	}
	if inode.isDirectory {
		content, err := f.getDirContent(name)
		_ = err // TODO
		if all {
			for _, dinode := range content {
				if err := f.remove(dinode.realName, true); err != nil {
					return err
				}
			}
		} else {
			if len(content) != 0 {
				return MakeError("Remove", name, "directory is not empty")
			}
		}
	}
	delete(f.inodes, name)
	return nil

}

func (f *FakeFS) Rename(oldpath, newpath string) error {
	oldpath = f.normilizePath(oldpath)
	newpath = f.normilizePath(newpath)
	inode, ok := f.inodes[oldpath]
	if !ok {
		return MakeWrappedError("Rename", oldpath, os.ErrNotExist)
	}

	targetDir := filepath.Dir(newpath)
	targetDirNode, ok := f.inodes[targetDir]
	if !ok {
		return &os.LinkError{Op: "Rename", Old: oldpath, New: newpath, Err: os.ErrNotExist}
	}

	if !targetDirNode.isDirectory {
		return &os.LinkError{Op: "Rename", Old: oldpath, New: newpath, Err: os.ErrInvalid}
	}

	delete(f.inodes, oldpath)
	f.inodes[newpath] = inode
	inode.realName = newpath
	inode.parent = targetDirNode
	return nil
}

func (f *FakeFS) Truncate(name string, size int64) error {
	name = f.normilizePath(name)
	inode, ok := f.inodes[name]
	if !ok {
		return MakeWrappedError("Truncate", name, os.ErrNotExist)
	}
	if !inode.hasWritePerm() {
		return MakeWrappedError("Truncate", name, os.ErrPermission)
	}
	// TODO: check write permission
	inode.buff = util.ResizeSlice(inode.buff, int(size))
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
	name = f.normilizePath(name)
	inode, ok := f.inodes[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	// TODO: check read persmissions?
	info := NewInfoDataFromNode(inode, inode.realName)
	return info, nil
}

// This function will probably changed by v1.0
func (f *FakeFS) CorruptFile(path string, offset int64) error {
	fp, ok := f.inodes[path]
	if !ok {
		return os.ErrNotExist
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
			flipByte := seedRand.Int63n(dirtyInterval.to-dirtyInterval.from) + dirtyInterval.from
			if flipByte < int64(len(data.buff)) { // TODO: do I need this if?
				data.buff[flipByte]++
			}
		}
	}
}

func (f *FakeFS) getDirContent(path string) ([]*mockData, error) {
	if path == "." {
		path = f.workDir
	}

	// TODO: check if have
	var res []*mockData
	for _, node := range f.inodes {
		if node.parent == nil {
			continue
		}
		if node.parent.realName == path {
			res = append(res, node)
		}
	}
	return res, nil
}
