package gofs

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/myxo/gofs/internal/util"
)

type InMemoryFS struct {
	inodes          map[string]*memData
	workDir         string
	trackDirtyPages bool
	threadSafeMode  bool
	mu              sync.Mutex
}

var _ FS = &InMemoryFS{}

const rootDir = "/"

// NewMemoryFs create fake filesystem with gofs.FS interface
func NewMemoryFs() *InMemoryFS {
	ret := &InMemoryFS{
		inodes:  map[string]*memData{},
		workDir: rootDir,
	}
	ret.inodes[rootDir] = &memData{
		realName:    rootDir,
		isDirectory: true,
		perm:        0666,
		fs:          ret,
	}
	return ret
}

// NewThreadSafeMemoryFs create thread safe fake fs. It's a bit slower, but you can use it in multithreading tests with -race
func NewThreadSafeMemoryFs() *InMemoryFS {
	ret := NewMemoryFs()
	ret.threadSafeMode = true
	return ret
}

func (f *InMemoryFS) TrackDirtyPages() {
	if f.threadSafeMode {
		f.mu.Lock()
		defer f.mu.Unlock()
	}

	f.trackDirtyPages = true
}

func (f *InMemoryFS) Create(path string) (*File, error) {
	return f.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

// prefixAndSuffix splits pattern by the last wildcard "*", if applicable,
// returning prefix as the part before "*" and suffix as the part after "*".
func prefixAndSuffix(pattern string) (prefix, suffix string, err error) {
	for i := 0; i < len(pattern); i++ {
		if os.IsPathSeparator(pattern[i]) {
			return "", "", errors.New("pattern contains path separator")
		}
	}
	if pos := strings.LastIndexByte(pattern, '*'); pos != -1 {
		prefix, suffix = pattern[:pos], pattern[pos+1:]
	} else {
		prefix = pattern
	}
	return prefix, suffix, nil
}

func (f *InMemoryFS) CreateTemp(dir, pattern string) (*File, error) {
	if dir == "" {
		dir = rootDir
	}

	prefix, suffix, err := prefixAndSuffix(pattern)
	if err != nil {
		return nil, MakeWrappedError("CreateTemp", pattern, err)
	}
	prefix = filepath.Join(dir, prefix)

	try := 0
	for {
		random := strconv.Itoa(int(rand.Int31()))
		name := prefix + random + suffix
		// TODO: we have all keys in map, so we may optimize
		f, err := f.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(err) {
			if try++; try < 10000 {
				continue
			}
			return nil, MakeWrappedError("CreateTemp", pattern+"*"+suffix, os.ErrExist)
		}
		return f, err
	}
}

func (f *InMemoryFS) Open(name string) (*File, error) {
	return f.OpenFile(name, os.O_RDONLY, 0)
}

func checkOpenPerm(flag int, inode *memData) error {
	if util.HasWritePerm(flag) && !inode.hasWritePerm() {
		return os.ErrPermission
	}
	if util.HasReadPerm(flag) && !inode.hasReadPerm() {
		return os.ErrPermission
	}
	return nil
}

func (f *InMemoryFS) normilizePath(path string) string {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(f.workDir, path)
	}
	return path
}

func (f *InMemoryFS) OpenFile(name string, flag int, perm os.FileMode) (*File, error) {
	if f.threadSafeMode {
		f.mu.Lock()
		defer f.mu.Unlock()
	}

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
		inode = filePool.Get().(*memData)
		inode.reset()
		inode.realName = name
		inode.perm = perm
		inode.fs = f
		inode.parent = dir
		inode.threadSafeMode = f.threadSafeMode
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
			name:  name,
			data:  inode,
			flag:  flag,
			valid: true,
		},
	}, nil
}

func (f *InMemoryFS) Chdir(dir string) error {
	if f.threadSafeMode {
		f.mu.Lock()
		defer f.mu.Unlock()
	}

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

func (f *InMemoryFS) Chmod(name string, mode os.FileMode) error {
	if f.threadSafeMode {
		f.mu.Lock()
		defer f.mu.Unlock()
	}

	name = f.normilizePath(name)
	inode, ok := f.inodes[name]
	if !ok {
		return MakeWrappedError("Chmod", name, os.ErrNotExist)
	}
	inode.perm = mode & fs.ModePerm
	return nil
}

func (f *InMemoryFS) Chown(name string, uid, gid int) error {
	// fs ownership is not implemented
	return nil
}

func (f *InMemoryFS) Mkdir(name string, perm os.FileMode) error {
	if f.threadSafeMode {
		f.mu.Lock()
		defer f.mu.Unlock()
	}

	name = f.normilizePath(name)
	parentPath := filepath.Dir(name)
	parent, parentExist := f.inodes[parentPath]
	if !parentExist || !parent.isDirectory {
		return MakeWrappedError("Mkdir", name, os.ErrNotExist)
	}

	if _, exist := f.inodes[name]; exist {
		return MakeWrappedError("Mkdir", name, os.ErrExist)
	}

	inode := &memData{
		realName:    name,
		isDirectory: true,
		perm:        perm,
		fs:          f,
		parent:      parent,
	}
	f.inodes[name] = inode
	return nil
}

func (f *InMemoryFS) MkdirAll(path string, perm os.FileMode) error {
	if f.threadSafeMode {
		f.mu.Lock()
		defer f.mu.Unlock()
	}

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

	inode := &memData{
		realName:    path,
		isDirectory: true,
		perm:        perm,
		fs:          f,
		parent:      parent,
	}
	f.inodes[path] = inode
	return nil
}

func (f *InMemoryFS) MkdirTemp(dir, pattern string) (string, error) {
	if dir == "" {
		dir = rootDir
	}

	prefix, suffix, err := prefixAndSuffix(pattern)
	if err != nil {
		return "", MakeWrappedError("MkdirTemp", pattern, err)
	}
	prefix = filepath.Join(dir, prefix)

	try := 0
	for {
		random := strconv.Itoa(int(rand.Int31()))
		name := prefix + random + suffix
		err := f.Mkdir(name, 0700)
		if err == nil {
			return name, nil
		}
		if os.IsExist(err) {
			if try++; try < 10000 {
				continue
			}
			return "", MakeWrappedError("CreateTemp", pattern+"*"+suffix, os.ErrExist)
		}
		if os.IsNotExist(err) {
			if _, err := f.Stat(dir); os.IsNotExist(err) {
				return "", err
			}
		}
		return "", err
	}
}

func (f *InMemoryFS) TempDir() string {
	_ = f.MkdirAll("/tmp", 0777)
	return "/tmp"
}

func (f *InMemoryFS) ReadFile(name string) ([]byte, error) {
	fp, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	return io.ReadAll(fp)
}

func (f *InMemoryFS) ReadDir(name string) ([]os.DirEntry, error) {
	fp, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	dirs, err := fp.ReadDir(-1)
	slices.SortFunc(dirs, func(a os.DirEntry, b os.DirEntry) int { return cmp.Compare(a.Name(), b.Name()) })
	return dirs, err
}

func (f *InMemoryFS) Readlink(name string) (string, error) { panic("TODO") }

func (f *InMemoryFS) Remove(name string) error {
	if f.threadSafeMode {
		f.mu.Lock()
		defer f.mu.Unlock()
	}

	return f.remove(name, false)
}

func (f *InMemoryFS) RemoveAll(path string) error {
	if f.threadSafeMode {
		f.mu.Lock()
		defer f.mu.Unlock()
	}

	return f.remove(path, true)
}

func (f *InMemoryFS) remove(name string, all bool) error {
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

func (f *InMemoryFS) Rename(oldpath, newpath string) error {
	if f.threadSafeMode {
		f.mu.Lock()
		defer f.mu.Unlock()
	}

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

func (f *InMemoryFS) Truncate(name string, size int64) error {
	if f.threadSafeMode {
		f.mu.Lock()
		defer f.mu.Unlock()
	}

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

func (f *InMemoryFS) WriteFile(name string, data []byte, perm os.FileMode) error {
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
func (f *InMemoryFS) Release() {
	if f.threadSafeMode {
		f.mu.Lock()
		defer f.mu.Unlock()
	}

	for _, v := range f.inodes {
		if v != nil {
			filePool.Put(v)
		}
	}
	clear(f.inodes)
}

func (f *InMemoryFS) Stat(name string) (os.FileInfo, error) {
	if f.threadSafeMode {
		f.mu.Lock()
		defer f.mu.Unlock()
	}

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
func (f *InMemoryFS) CorruptFile(path string, offset int64) error {
	if f.threadSafeMode {
		f.mu.Lock()
		defer f.mu.Unlock()
	}

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
func (f *InMemoryFS) CorruptDirtyPages(seedRand *rand.Rand) {
	if f.threadSafeMode {
		f.mu.Lock()
		defer f.mu.Unlock()
	}

	for _, data := range f.inodes {
		for _, dirtyInterval := range data.dirtyPages {
			flipByte := seedRand.Int63n(dirtyInterval.to-dirtyInterval.from) + dirtyInterval.from
			if flipByte < int64(len(data.buff)) { // TODO: do I need this if?
				data.buff[flipByte]++
			}
		}
	}
}

func (f *InMemoryFS) getDirContent(path string) ([]*memData, error) {
	if path == "." {
		path = f.workDir
	}

	// TODO: check if have
	var res []*memData
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
