package memory

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/myxo/gofs"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func checkSyncError(t *rapid.T, errOs error, errFake error) {
	t.Helper()

	if errOs == io.EOF {
		// EOF is special, since it often used as signal of stop (e.g. io.Copy), so we must return io.EOF when os does
		if errFake != io.EOF {
			t.Fatalf("os return io.EOF, but fake return %q", errFake)
		}
		return
	}

	if os.IsExist(errOs) != os.IsExist(errFake) {
		t.Fatalf("os and fake impl tread os.IsExist differently os:%q fake=%q", errOs, errFake)
	}
	if os.IsNotExist(errOs) != os.IsNotExist(errFake) {
		t.Fatalf("os and fake impl tread os.IsNotExist differently os:%q fake=%q", errOs, errFake)
	}
	if os.IsPermission(errOs) != os.IsPermission(errFake) {
		t.Fatalf("os and fake impl tread os.IsPermission differently os:%q fake=%q", errOs, errFake)
	}
	if (errOs != nil) != (errFake != nil) {
		t.Fatalf("os and fake impl produce different error os:%q fake=%q", errOs, errFake)
	}
}

/*
func listOsFiles(dirPath string) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		fmt.Println(file.Name(), file.IsDir())
	}
}
*/

func TestFS(t *testing.T) {
	dir := t.TempDir()

	resetStat()
	var memStat runtime.MemStats
	runtime.ReadMemStats(&memStat)

	rapid.Check(t, func(t *rapid.T) {
		fs := &statFs{fs: gofs.NewMemoryFs()}
		possibleFilenames := []string{"/foo/a/test.file.1", "/foo/a/test.file.2", "/foo/b/test.file.1", "/foo/b/test.file.2"}
		possibleDirs := []string{"/foo", "/foo/a", "/foo/b"}
		var osFiles []*gofs.File
		var fakeFiles []*statFile
		workDir := "/"
		err := os.Chdir(dir)
		require.NoError(t, err)

		defer func() {
			for i := range osFiles {
				osFiles[i].Close()
			}
			_ = os.RemoveAll(filepath.Join(dir, "foo"))
			fs.fs.Release()
		}()

		require.NoError(t, os.MkdirAll(filepath.Join(dir, "foo/a"), 0777))
		require.NoError(t, fs.MkdirAll("/foo/a", 0777))
		{
			// create first file, so we don't spend first iterations just on errors
			fpOs, err := os.Create(filepath.Join(dir, possibleFilenames[0]))
			require.NoError(t, err)
			fpFake, err := fs.Create(filepath.Join("/", possibleFilenames[0]))
			require.NoError(t, err)

			osFiles = append(osFiles, gofs.NewFromOs(fpOs))
			fakeFiles = append(fakeFiles, &statFile{fp: fpFake})
		}

		getFiles := func() (*gofs.File, *statFile) {
			i := rapid.IntRange(0, len(osFiles)-1).Draw(t, "file index")
			return osFiles[i], fakeFiles[i]
		}

		getFilePaths := func() (string, string) {
			p := rapid.SampledFrom(possibleFilenames).Draw(t, "path")
			osAbs := filepath.Join(dir, p)
			fakeAbs := filepath.Join("/", p)
			if !rapid.Bool().Draw(t, "relative path") {
				return osAbs, fakeAbs
			}
			fakeRel, err := filepath.Rel(workDir, fakeAbs)
			require.NoError(t, err)
			osWorkDir := filepath.Join(dir, workDir) // TODO: save var to not allocate?
			osRel, err := filepath.Rel(osWorkDir, osAbs)
			require.NoError(t, err)
			return osRel, fakeRel
		}

		getDirPaths := func() (string, string) {
			p := rapid.SampledFrom(possibleDirs).Draw(t, "path")
			osP := filepath.Join(dir, p)
			fakeP := filepath.Join("/", p)
			return osP, fakeP
		}

		t.Repeat(map[string]func(*rapid.T){
			"write": func(t *rapid.T) {
				fpOs, fpFake := getFiles()
				n := rapid.IntRange(0, 1024).Draw(t, "write size")
				buff := make([]byte, n)
				_, _ = rand.Read(buff)
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
				_, _ = rand.Read(buff)
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
			"Chdir": func(t *rapid.T) {
				fpOs, fpFake := getFiles()
				errOs := fpOs.Chdir()
				errFake := fpFake.Chdir()
				checkSyncError(t, errOs, errFake)
			},
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
				CompareDirEntries(t, diOs, diFake)
			},
			"ReadFrom": func(t *rapid.T) {
				fpOs, fpFake := getFiles()
				n := rapid.IntRange(0, 1024).Draw(t, "write size")
				buff := make([]byte, n)
				_, _ = rand.Read(buff)
				nOs, errOs := fpOs.ReadFrom(bytes.NewReader(buff))
				nFake, errFake := fpFake.ReadFrom(bytes.NewReader(buff))
				checkSyncError(t, errOs, errFake)
				require.Equal(t, nOs, nFake)
			},
			/*
				"WriteTo": func(t *rapid.T) {
					fpOs, fpFake := getFiles()
					var buffOs bytes.Buffer
					var buffFake bytes.Buffer
					nOs, errOs := fpOs.WriteTo(&buffOs)
					nFake, errFake := fpFake.WriteTo(&buffFake)
					checkSyncError(t, errOs, errFake)
					require.Equal(t, nOs, nFake)
					require.Equal(t, buffOs.Bytes(), buffFake.Bytes())
				},
			*/
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
				osPath, fakePath := getFilePaths()
				fpOs, errOs := os.Create(osPath)
				fpFake, errFake := fs.Create(fakePath)
				checkSyncError(t, errOs, errFake)
				if errOs == nil {
					osFiles = append(osFiles, gofs.NewFromOs(fpOs))
					fakeFiles = append(fakeFiles, &statFile{fp: fpFake})
				}
			},
			"FS_Open": func(t *rapid.T) {
				osPath, fakePath := getFilePaths()
				fpOs, errOs := os.Open(osPath)
				fpFake, errFake := fs.Open(fakePath)
				checkSyncError(t, errOs, errFake)
				if errOs == nil {
					osFiles = append(osFiles, gofs.NewFromOs(fpOs))
					fakeFiles = append(fakeFiles, &statFile{fp: fpFake})
				}
			},
			"FS_OpenFile": func(t *rapid.T) {
				osPath, fakePath := getFilePaths()
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

				fpOs, errOs := os.OpenFile(osPath, flag, perm)
				fpFake, errFake := fs.OpenFile(fakePath, flag, perm)
				checkSyncError(t, errOs, errFake)
				if errOs == nil {
					osFiles = append(osFiles, gofs.NewFromOs(fpOs))
					fakeFiles = append(fakeFiles, &statFile{fp: fpFake})
				}
			},
			"FS_Chdir": func(t *rapid.T) {
				osPath, fakePath := getDirPaths()
				errOs := os.Chdir(osPath)
				errFake := fs.Chdir(fakePath)
				checkSyncError(t, errOs, errFake)
			},
			"FS_Chmod": func(t *rapid.T) {
				osPath, fakePath := getFilePaths()
				possibleModes := []os.FileMode{0666, 0222, 0444} // rw, w-only, r-only
				mode := rapid.SampledFrom(possibleModes).Draw(t, "file mode")
				errOs := os.Chmod(osPath, mode)
				errFake := fs.Chmod(fakePath, mode)
				checkSyncError(t, errOs, errFake)
			},
			"FS_Mkdir": func(t *rapid.T) {
				osPath, fakePath := getDirPaths()
				errOs := os.Mkdir(osPath, 0777)
				errFake := fs.Mkdir(fakePath, 0777)
				checkSyncError(t, errOs, errFake)
			},
			"FS_MkdirAll": func(t *rapid.T) {
				osPath, fakePath := getDirPaths()
				errOs := os.MkdirAll(osPath, 0777)
				errFake := fs.MkdirAll(fakePath, 0777)
				checkSyncError(t, errOs, errFake)
			},
			"FS_ReadFile": func(t *rapid.T) {
				osPath, fakePath := getFilePaths()
				contOs, errOs := os.ReadFile(osPath)
				contFake, errFake := fs.ReadFile(fakePath)
				checkSyncError(t, errOs, errFake)
				require.Equal(t, contOs, contFake)
			},
			"FS_Remove": func(t *rapid.T) {
				// TODO: remove also dirs and subdirs
				osPath, fakePath := getFilePaths()
				errOs := os.Remove(osPath)
				errFake := fs.Remove(fakePath)
				checkSyncError(t, errOs, errFake)
			},
			"FS_RemoveAll": func(t *rapid.T) {
				// TODO: remove also dirs and subdirs
				osPath, fakePath := getFilePaths()
				errOs := os.RemoveAll(osPath)
				errFake := fs.RemoveAll(fakePath)
				checkSyncError(t, errOs, errFake)
			},
			"FS_Rename": func(t *rapid.T) {
				oldOsPath, oldFakePath := getFilePaths()
				newOsPath, newFakePath := getFilePaths()

				errOs := os.Rename(oldOsPath, newOsPath)
				errFake := fs.Rename(oldFakePath, newFakePath)
				checkSyncError(t, errOs, errFake)
			},
			"FS_Truncate": func(t *rapid.T) {
				osPath, fakePath := getFilePaths()
				n := rapid.Int64Range(0, 1024).Draw(t, "truncate size")
				errOs := os.Truncate(osPath, n)
				errFake := fs.Truncate(fakePath, n)
				checkSyncError(t, errOs, errFake)
			},
			"FS_WriteFile": func(t *rapid.T) {
				osPath, fakePath := getFilePaths()
				n := rapid.IntRange(0, 1024).Draw(t, "write size")
				buff := make([]byte, n)
				_, _ = rand.Read(buff)
				errOs := os.WriteFile(osPath, buff, 0777)
				errFake := fs.WriteFile(fakePath, buff, 0777)
				checkSyncError(t, errOs, errFake)
			},
			"FS_Stat": func(t *rapid.T) {
				osPath, fakePath := getFilePaths()
				fiOs, errOs := os.Stat(osPath)
				fiFake, errFake := fs.Stat(fakePath)
				checkSyncError(t, errOs, errFake)
				if fiOs != nil {
					CompareFileInfo(t, fiOs, fiFake)
				}
			},
			"FS_ReadDir": func(t *rapid.T) {
				osPath, fakePath := getDirPaths()
				diOs, errOs := os.ReadDir(osPath)
				diFake, errFake := fs.ReadDir(fakePath)
				checkSyncError(t, errOs, errFake)
				CompareDirEntries(t, diOs, diFake)
			},
			"FS_CreateTemp": func(t *rapid.T) {
				osPath, fakePath := getDirPaths()
				fpOs, errOs := os.CreateTemp(osPath, "temp*.txt")
				fpFake, errFake := fs.CreateTemp(fakePath, "temp*.txt")
				checkSyncError(t, errOs, errFake)
				if errOs != nil {
					return
				}
				newName := filepath.Join(filepath.Dir(fpOs.Name()), filepath.Base(fpFake.Name()))
				err := os.Rename(fpOs.Name(), newName)
				require.NoError(t, err)
				possibleFilenames = append(possibleFilenames, fpFake.Name())
			},
			"FS_MkdirTemp": func(t *rapid.T) {
				osPath, fakePath := getDirPaths()
				pathOs, errOs := os.MkdirTemp(osPath, "dir*")
				pathFake, errFake := fs.MkdirTemp(fakePath, "dir*")
				checkSyncError(t, errOs, errFake)
				if errOs != nil {
					return
				}
				newName := filepath.Join(dir, pathFake)
				err := os.Rename(pathOs, newName)
				require.NoError(t, err)
				possibleDirs = append(possibleDirs, pathFake)
			},
		})
	})

	printStat()
	var memStatAfter runtime.MemStats
	runtime.ReadMemStats(&memStatAfter)
	fmt.Printf("Mallocs: %d\n", memStatAfter.Mallocs-memStat.Mallocs)
}

func CompareFileInfo(t *rapid.T, fiOs os.FileInfo, fiFake os.FileInfo) {
	require.Equal(t, fiOs.Name(), fiFake.Name())
	require.Equal(t, fiOs.IsDir(), fiFake.IsDir())
	require.Equal(t, fiOs.Size(), fiFake.Size())
	// we do not compare mode, since it depends on parent fs directory, so hard to check in property test
	// We do not compare time, since it's hard to mock, and not really relevant
}

func CompareDirEntries(t *rapid.T, diOs []os.DirEntry, diFake []os.DirEntry) {
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
}

func TestStandardErrorChecks(t *testing.T) {
	t.Run("not exist", func(t *testing.T) {
		fs := gofs.NewMemoryFs()
		_, err := fs.OpenFile("test", os.O_RDWR, 0666)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("exist", func(t *testing.T) {
		fs := gofs.NewMemoryFs()
		fp, err := fs.OpenFile("test", os.O_CREATE|os.O_EXCL, 0666)
		require.NoError(t, err)
		_ = fp.Close()
		_, err = fs.OpenFile("test", os.O_CREATE|os.O_EXCL, 0666)
		require.True(t, os.IsExist(err))
	})
}
