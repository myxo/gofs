package gofs

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func checkSyncError(t *rapid.T, errOs error, errFake error) {
	t.Helper()
	if (errOs != nil) != (errFake != nil) {
		t.Fatalf("os and fake impl produce different error os:%q fake=%q", errOs, errFake)
	}
}

func listOsFiles(dirPath string) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		fmt.Println(file.Name(), file.IsDir())
	}
}

func TestFS(t *testing.T) {
	dir := t.TempDir()
	possibleFilenames := []string{"/a/test.file.1", "/a/test.file.2", "/b/test.file.1", "/b/test.file.2"}
	possibleDirs := []string{"/a", "/b"}

	rapid.Check(t, func(t *rapid.T) {
		fs := NewMemoryFs()
		var osFiles []*File
		var fakeFiles []*File
		fileCount := 0

		defer func() {
			for i := range osFiles {
				osFiles[i].Close()
			}
			os.RemoveAll(filepath.Join(dir, "a"))
			os.RemoveAll(filepath.Join(dir, "b"))
			fs.Release()
		}()
		err := fs.Mkdir("/a", 0777)
		require.NoError(t, err)
		err = os.MkdirAll(filepath.Join(dir, "a"), 0777)
		require.NoError(t, err)
		createFiles := func() {
			fpOs, err := os.Create(filepath.Join(dir, possibleFilenames[0]))
			require.NoError(t, err)
			fpFake, err := fs.Create(filepath.Join("/", possibleFilenames[0]))
			require.NoError(t, err)

			osFiles = append(osFiles, NewFromOs(fpOs))
			fakeFiles = append(fakeFiles, fpFake)
			fileCount++
		}
		createFiles()

		getFiles := func() (*File, *File) {
			i := rapid.IntRange(0, len(osFiles)-1).Draw(t, "file index")
			return osFiles[i], fakeFiles[i]
		}

		t.Repeat(map[string]func(*rapid.T){
			"write": func(t *rapid.T) {
				fpOs, fpFake := getFiles()
				n := rapid.IntRange(0, 1024).Draw(t, "write size")
				buff := make([]byte, n)
				rand.Read(buff)
				nOs, errOs := fpOs.Write(buff)
				nFake, errFake := fpFake.Write(buff)
				checkSyncError(t, errOs, errFake)
				if nOs != nFake {
					t.Fatalf("os impl return %d, we %d", nOs, nFake)
				}
			},
			"writeAt": func(t *rapid.T) {
				fpOs, fpFake := getFiles()
				offset := rapid.Int64Range(-1, 1024).Draw(t, "write at offset")
				n := rapid.IntRange(0, 1024).Draw(t, "write at size")
				buff := make([]byte, n)
				rand.Read(buff)
				nOs, errOs := fpOs.WriteAt(buff, offset)
				nFake, errFake := fpFake.WriteAt(buff, offset)
				checkSyncError(t, errOs, errFake)
				if nOs != nFake {
					t.Fatalf("os impl return %d, we %d", nOs, nFake)
				}
			},
			"read": func(t *rapid.T) {
				fpOs, fpFake := getFiles()
				n := rapid.IntRange(0, 10*1024).Draw(t, "read size")
				buffOs := make([]byte, n)
				buffFake := make([]byte, n)
				nOs, errOs := fpOs.Read(buffOs)
				nFake, errFake := fpFake.Read(buffFake)
				checkSyncError(t, errOs, errFake)
				require.Equal(t, nOs, nFake)
				require.Equal(t, buffOs[:nOs], buffFake[:nFake])
			},
			"readAt": func(t *rapid.T) {
				fpOs, fpFake := getFiles()
				offset := rapid.Int64().Draw(t, "read at offset")
				n := rapid.IntRange(0, 10*1024).Draw(t, "read size")
				buffOs := make([]byte, n)
				buffFake := make([]byte, n)
				nOs, errOs := fpOs.ReadAt(buffOs, offset)
				nFake, errFake := fpFake.ReadAt(buffFake, offset)
				checkSyncError(t, errOs, errFake)
				require.Equal(t, nOs, nFake)
				require.Equal(t, buffOs[:nOs], buffFake[:nFake])
			},
			"Chmod": func(t *rapid.T) {
				// we do not check execute permission
				// we do not check permission for different user groups
				possibleModes := []os.FileMode{0666, 0222, 0444} // rw, w-only, r-only
				fpOs, fpFake := getFiles()
				mode := rapid.SampledFrom(possibleModes).Draw(t, "file mode")
				errOs := fpOs.Chmod(mode)
				errFake := fpFake.Chmod(mode)
				checkSyncError(t, errOs, errFake)
			},
			//			//"Chown": func(t *rapid.T) {}, // TODO: ??
			//			//"Chdir": func(t *rapid.T) {}, // TODO: ??
			"Close": func(t *rapid.T) {
				fpOs, fpFake := getFiles()
				errOs := fpOs.Close()
				errFake := fpFake.Close()
				checkSyncError(t, errOs, errFake)
			},
			"ReadDir": func(t *rapid.T) {
				fpOs, fpFake := getFiles()
				n := rapid.IntRange(-1, 3).Draw(t, "readDir n")
				diOs, errOs := fpOs.ReadDir(n)
				diFake, errFake := fpFake.ReadDir(n)
				checkSyncError(t, errOs, errFake)
				slices.SortFunc(diOs, func(a os.DirEntry, b os.DirEntry) int { return cmp.Compare(a.Name(), b.Name()) })
				slices.SortFunc(diFake, func(a os.DirEntry, b os.DirEntry) int { return cmp.Compare(a.Name(), b.Name()) })
				require.Equal(t, len(diOs), len(diFake))
				for i := range diOs {
					require.Equal(t, diOs[i].Name(), diFake[i].Name())
					require.Equal(t, diOs[i].IsDir(), diFake[i].IsDir())
					infoOs, errOs := diOs[i].Info()
					infoFake, errFake := diFake[i].Info()
					checkSyncError(t, errOs, errFake)
					if errOs != nil {
						require.Equal(t, infoOs.Size(), infoFake.Size())
						require.Equal(t, infoOs.Mode(), infoFake.Mode())
					}
				}
			},
			"ReadFrom": func(t *rapid.T) {
				fpOs, fpFake := getFiles()
				var buffOs bytes.Buffer
				var buffFake bytes.Buffer
				nOs, errOs := fpOs.ReadFrom(&buffOs)
				nFake, errFake := fpFake.ReadFrom(&buffFake)
				checkSyncError(t, errOs, errFake)
				require.Equal(t, nOs, nFake)
				require.Equal(t, buffOs.Bytes(), buffFake.Bytes())
			},
			"Readdir": func(t *rapid.T) {
				fpOs, fpFake := getFiles()
				n := rapid.IntRange(-1, 3).Draw(t, "readdir n")
				infoOs, errOs := fpOs.Readdir(n)
				infoFake, errFake := fpFake.Readdir(n)
				checkSyncError(t, errOs, errFake)
				slices.SortFunc(infoOs, func(a os.FileInfo, b os.FileInfo) int { return cmp.Compare(a.Name(), b.Name()) })
				slices.SortFunc(infoFake, func(a os.FileInfo, b os.FileInfo) int { return cmp.Compare(a.Name(), b.Name()) })
				require.Equal(t, len(infoOs), len(infoFake))
				for i := range infoOs {
					require.Equal(t, infoOs[i].Name(), infoFake[i].Name())
					require.Equal(t, infoOs[i].IsDir(), infoFake[i].IsDir())
					require.Equal(t, infoOs[i].Size(), infoFake[i].Size())
					require.Equal(t, infoOs[i].Mode(), infoFake[i].Mode())
				}
			},
			"Readdirnames": func(t *rapid.T) {
				fpOs, fpFake := getFiles()
				n := rapid.IntRange(-1, 3).Draw(t, "readdir n")
				infoOs, errOs := fpOs.Readdirnames(n)
				infoFake, errFake := fpFake.Readdirnames(n)
				checkSyncError(t, errOs, errFake)
				slices.Sort(infoOs)
				slices.Sort(infoFake)
				require.Equal(t, infoOs, infoFake)
			},
			"Seek": func(t *rapid.T) {
				fpOs, fpFake := getFiles()
				offset := rapid.Int64Range(-1, 1024).Draw(t, "seek offset")
				whence := rapid.SampledFrom([]int{io.SeekStart, io.SeekCurrent, io.SeekEnd}).Draw(t, "seek whence")
				retOs, errOs := fpOs.Seek(offset, whence)
				retFake, errFake := fpFake.Seek(offset, whence)
				checkSyncError(t, errOs, errFake)
				require.Equal(t, retOs, retFake)
			},
			"Stat": func(t *rapid.T) {
				fpOs, fpFake := getFiles()
				fiOs, errOs := fpOs.Stat()
				fiFake, errFake := fpFake.Stat()
				checkSyncError(t, errOs, errFake)
				if fiOs != nil {
					CompareFileInfo(t, fiOs, fiFake)
				}
			},
			"Sync": func(t *rapid.T) {
				fpOs, fpFake := getFiles()
				errOs := fpOs.Sync()
				errFake := fpFake.Sync()
				checkSyncError(t, errOs, errFake)
			},
			"Truncate": func(t *rapid.T) {
				fpOs, fpFake := getFiles()
				offset := rapid.Int64Range(-1, 1024).Draw(t, "truncate offset")
				errOs := fpOs.Truncate(offset)
				errFake := fpFake.Truncate(offset)
				checkSyncError(t, errOs, errFake)
			},
			"FS_Create": func(t *rapid.T) {
				p := rapid.SampledFrom(possibleFilenames).Draw(t, "file to create")
				fpOs, errOs := os.Create(filepath.Join(dir, p))
				fpFake, errFake := fs.Create(filepath.Join("/", p))
				checkSyncError(t, errOs, errFake)
				if errOs == nil {
					osFiles = append(osFiles, NewFromOs(fpOs))
					fakeFiles = append(fakeFiles, fpFake)
				}
			},
			//			"FS_CreateTemp": func(t *rapid.T) {},
			"FS_Open": func(t *rapid.T) {
				p := rapid.SampledFrom(possibleFilenames).Draw(t, "file to create")
				fpOs, errOs := os.Open(filepath.Join(dir, p))
				fpFake, errFake := fs.Open(filepath.Join("/", p))
				checkSyncError(t, errOs, errFake)
				if errOs == nil {
					osFiles = append(osFiles, NewFromOs(fpOs))
					fakeFiles = append(fakeFiles, fpFake)
				}
			},
			"FS_OpenFile": func(t *rapid.T) {
				p := rapid.SampledFrom(possibleFilenames).Draw(t, "file to open")
				possibleModes := []os.FileMode{0666, 0222, 0444} // rw, w-only, r-only
				perm := rapid.SampledFrom(possibleModes).Draw(t, "file perm")
				flagMap := map[string]int{"readonly": os.O_RDONLY, "writeonly": os.O_WRONLY, "RDWR": os.O_RDWR}
				possibleFlags := []string{"readonly", "writeonly", "RDWR"}
				flag := flagMap[rapid.SampledFrom(possibleFlags).Draw(t, "file mode")]
				//if rapid.Bool().Draw(t, "append bit") {
				//	flag |= os.O_APPEND
				//}
				if rapid.Bool().Draw(t, "create bit") {
					flag |= os.O_CREATE
				}
				if rapid.Bool().Draw(t, "excl bit") {
					flag |= os.O_EXCL
				}
				if rapid.Bool().Draw(t, "trunc bit") {
					flag |= os.O_TRUNC
				}

				fpOs, errOs := os.OpenFile(filepath.Join(dir, p), flag, perm)
				fpFake, errFake := fs.OpenFile(filepath.Join("/", p), flag, perm)
				checkSyncError(t, errOs, errFake)
				if errOs == nil {
					osFiles = append(osFiles, NewFromOs(fpOs))
					fakeFiles = append(fakeFiles, fpFake)
				}
			},
			//			"FS_Chdir":      func(t *rapid.T) {},
			"FS_Chmod": func(t *rapid.T) {
				p := rapid.SampledFrom(possibleFilenames).Draw(t, "file to create")
				fpOs, errOs := os.Open(filepath.Join(dir, p))
				fpFake, errFake := fs.Open(filepath.Join("/", p))
				checkSyncError(t, errOs, errFake)
				if errOs == nil {
					possibleModes := []os.FileMode{0666, 0222, 0444} // rw, w-only, r-only
					mode := rapid.SampledFrom(possibleModes).Draw(t, "file mode")
					errOs := fpOs.Chmod(mode)
					errFake := fpFake.Chmod(mode)
					checkSyncError(t, errOs, errFake)
				}
			},
			//			"FS_Chown":      func(t *rapid.T) {},
			"FS_Mkdir": func(t *rapid.T) {
				p := rapid.SampledFrom(possibleDirs).Draw(t, "dir")
				errOs := os.Mkdir(filepath.Join(dir, p), 0777)
				errFake := fs.Mkdir(filepath.Join("/", p), 0777)
				checkSyncError(t, errOs, errFake)
			},
			"FS_MkdirAll": func(t *rapid.T) {
				p := rapid.SampledFrom(possibleDirs).Draw(t, "dir")
				errOs := os.MkdirAll(filepath.Join(dir, p), 0777)
				errFake := fs.MkdirAll(filepath.Join("/", p), 0777)
				checkSyncError(t, errOs, errFake)
			},
			//			"FS_MkdirTemp":  func(t *rapid.T) {},
			"FS_ReadFile": func(t *rapid.T) {
				p := rapid.SampledFrom(possibleFilenames).Draw(t, "file")
				contOs, errOs := os.ReadFile(filepath.Join(dir, p))
				contFake, errFake := fs.ReadFile(filepath.Join("/", p))
				checkSyncError(t, errOs, errFake)
				require.Equal(t, contOs, contFake)
			},
			"FS_Remove": func(t *rapid.T) {
				// TODO: remove also dirs and subdirs
				p := rapid.SampledFrom(possibleFilenames).Draw(t, "dir")
				errOs := os.Remove(filepath.Join(dir, p))
				errFake := fs.Remove(filepath.Join("/", p))
				checkSyncError(t, errOs, errFake)
			},
			"FS_RemoveAll": func(t *rapid.T) {
				// TODO: remove also dirs and subdirs
				p := rapid.SampledFrom(possibleFilenames).Draw(t, "dir")
				errOs := os.RemoveAll(filepath.Join(dir, p))
				errFake := fs.RemoveAll(filepath.Join("/", p))
				checkSyncError(t, errOs, errFake)
			},
			"FS_Rename": func(t *rapid.T) {
				oldname := rapid.SampledFrom(possibleFilenames).Draw(t, "file")
				newname := rapid.SampledFrom(possibleFilenames).Draw(t, "file")

				oldOsPath := filepath.Join(dir, oldname)
				newOsPath := filepath.Join(dir, newname)
				oldFakePath := filepath.Join("/", oldname)
				newFakePath := filepath.Join("/", newname)

				errOs := os.Rename(oldOsPath, newOsPath)
				errFake := fs.Rename(oldFakePath, newFakePath)
				checkSyncError(t, errOs, errFake)
			},
			"FS_Truncate": func(t *rapid.T) {
				p := rapid.SampledFrom(possibleFilenames).Draw(t, "file")
				n := rapid.Int64Range(0, 1024).Draw(t, "truncate size")
				errOs := os.Truncate(filepath.Join(dir, p), n)
				errFake := fs.Truncate(filepath.Join("/", p), n)
				checkSyncError(t, errOs, errFake)
			},
			"FS_WriteFile": func(t *rapid.T) {
				p := rapid.SampledFrom(possibleFilenames).Draw(t, "file")
				n := rapid.IntRange(0, 1024).Draw(t, "write size")
				buff := make([]byte, n)
				rand.Read(buff)
				errOs := os.WriteFile(filepath.Join(dir, p), buff, 0777)
				errFake := fs.WriteFile(filepath.Join("/", p), buff, 0777)
				checkSyncError(t, errOs, errFake)
			},
			"FS_Stat": func(t *rapid.T) {
				p := rapid.SampledFrom(possibleFilenames).Draw(t, "file")
				fiOs, errOs := os.Stat(filepath.Join(dir, p))
				fiFake, errFake := fs.Stat(filepath.Join("/", p))
				checkSyncError(t, errOs, errFake)
				if fiOs != nil {
					CompareFileInfo(t, fiOs, fiFake)
				}
			},
		})
	})
}

func CompareFileInfo(t *rapid.T, fiOs os.FileInfo, fiFake os.FileInfo) {
	require.Equal(t, fiOs.Name(), fiFake.Name())
	require.Equal(t, fiOs.IsDir(), fiFake.IsDir())
	require.Equal(t, fiOs.Size(), fiFake.Size())
	// we do not compare mode, since it depends on parent fs directory, so hard to check in property test
	// We do not compare time, since it's hard to mock, and not really relevant
}

func TestTmp(t *testing.T) {
	dir := t.TempDir()
	fs := NewMemoryFs()
	var osFiles []*File
	var fakeFiles []*File
	fileCount := 0

	defer func() {
		for i := range osFiles {
			osFiles[i].Close()
		}
		os.RemoveAll(filepath.Join(dir, "a"))
		os.RemoveAll(filepath.Join(dir, "b"))
		fs.Release()
	}()
	err := fs.Mkdir("/a", 0777)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(dir, "a"), 0777)
	require.NoError(t, err)
	createFiles := func() {
		fpOs, err := os.Create(filepath.Join(dir, "/a/file.test"))
		require.NoError(t, err)
		fpFake, err := fs.Create(filepath.Join("/", "/a/file.test"))
		require.NoError(t, err)

		osFiles = append(osFiles, NewFromOs(fpOs))
		fakeFiles = append(fakeFiles, fpFake)
		fileCount++
	}
	createFiles()

	fmt.Println("STAT")
	_, err1 := os.Stat(filepath.Join(dir, "/a/file.test"))
	_, err2 := fs.Stat(filepath.Join("/", "/a/file.test"))
	fmt.Println(err1)
	fmt.Println(err2)
	panic("aaa")

	p := "/a/file.test"
	fpOs, errOs := os.Open(filepath.Join(dir, p))
	fpFake, errFake := fs.Open(filepath.Join("/", p))
	fmt.Printf("oserr:%q, fakeErr:%q\n", errOs, errFake)

	fmt.Println("chmod")
	mode := os.FileMode(0222)
	errOs = fpOs.Chmod(mode)
	errFake = fpFake.Chmod(mode)
	fmt.Printf("oserr:%q, fakeErr:%q\n", errOs, errFake)

	fmt.Println("writeat")
	offset := int64(0)
	n := 1
	buff := make([]byte, n)
	rand.Read(buff)
	nOs, errOs := fpOs.WriteAt(buff, offset)
	nFake, errFake := fpFake.WriteAt(buff, offset)
	fmt.Printf("oserr:%q, fakeErr:%q\n", errOs, errFake)

	fmt.Println("FS_create")
	fpOs, errOs = os.Create(filepath.Join(dir, p))
	fpFake, errFake = fs.Create(filepath.Join("/", p))
	fmt.Printf("oserr:%q, fakeErr:%q\n", errOs, errFake)

	n = 1
	buff = make([]byte, n)
	rand.Read(buff)
	nOs, errOs = fpOs.Write(nil)
	nFake, errFake = fpFake.Write(nil)
	fmt.Printf("oserr:%q, fakeErr:%q\n", errOs, errFake)
	if nOs != nFake {
		t.Fatalf("os impl return %d, we %d", nOs, nFake)
	}
}

func TestTmp2(t *testing.T) {
	dir := t.TempDir()
	//dir := "/Users/myxo/tmp"
	path := filepath.Join(dir, "test2")
	fp, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0444)
	fmt.Println("open", err)
	buff := []byte("hello")
	_, err = fp.Read(buff)
	fmt.Println("read", err)
	_, err = fp.WriteAt(buff, 0)
	fmt.Println("write", err)
	n, err := fp.ReadAt(buff, 0)
	fmt.Println("read", n, err, buff[:n])
	fmt.Println("close")
	fp.Close()
	fp, err = os.OpenFile(path, os.O_RDONLY, 0222)
	fmt.Println("open", err)
	n, err = fp.ReadAt(buff, 0)
	fmt.Println("read", n, err, buff[:n])

}
