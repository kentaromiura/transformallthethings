// +build darwin freebsd netbsd openbsd linux
// @kentaromiura:
// transform.go reuses most of the passthrough.go example code,
// therefore follows original copyright/license.
/*
 * passthrough.go
 *
 * Copyright 2017-2020 Bill Zissimopoulos
 *
 * This file is part of Cgofuse.
 *
 * It is licensed under the MIT license. The full license text can be found
 * in the License.txt file at the root of this project.
 */

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"

	"bytes"
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/billziss-gh/cgofuse/fuse"
)

var transformMap map[string]interface{}

func errno(err error) int {
	if nil != err {
		return -int(err.(syscall.Errno))
	}
	return 0
}

var (
	_host *fuse.FileSystemHost
)

type ptfs struct {
	fuse.FileSystemBase
	root     string
	original string
	cache    string
}

func (vfs *ptfs) Init() {
	//  defer trace()()
	e := syscall.Chdir(vfs.root)
	if nil == e {
		vfs.root = "./"
	}
}

func (vfs *ptfs) Statfs(path string, stat *fuse.Statfs_t) (errc int) {
	//  defer trace(path)(&errc, stat)
	path = filepath.Join(vfs.root, path)
	stgo := syscall.Statfs_t{}
	errc = errno(syscall_Statfs(path, &stgo))
	copyFusestatfsFromGostatfs(stat, &stgo)
	return
}

func (vfs *ptfs) Readlink(path string) (errc int, target string) {
	//  defer trace(path)(&errc, &target)
	path = filepath.Join(vfs.root, path)
	buff := [1024]byte{}
	n, e := syscall.Readlink(path, buff[:])
	if nil != e {
		return errno(e), ""
	}
	return 0, string(buff[:n])
}

func (vfs *ptfs) Open(path string, flags int) (errc int, fh uint64) {
	//  defer trace(path, flags)(&errc, &fh)
	return vfs.open(path, flags, 0)
}

func (vfs *ptfs) open(path string, flags int, mode uint32) (errc int, fh uint64) {
	cachePath := filepath.Join(vfs.cache, path)
	if inCache, err := os.Open(cachePath); err == nil {
		defer inCache.Close()
		f, e := syscall.Open(cachePath, flags, mode)
		if nil != e {
			return errno(e), ^uint64(0)
		}
		return 0, uint64(f)
	}
	path = filepath.Join(vfs.root, path)
	f, e := syscall.Open(path, flags, mode)
	if nil != e {
		return errno(e), ^uint64(0)
	}
	return 0, uint64(f)
}

func (vfs *ptfs) Getattr(path string, stat *fuse.Stat_t, fh uint64) (errc int) {
	// defer trace(path, fh)(&errc, stat)
	// Since filesize can change after a transform we need to handle it here.
	for regex, exe := range transformMap {
		// TODO: precompiling all the regexes
		// should be better for performances.
		var matcher = regexp.MustCompile(regex)
		if matcher.MatchString(path) {
			cachePath := filepath.Join(vfs.cache, path)

			stgo := syscall.Stat_t{}
			realPath := filepath.Join(vfs.original, path)
			errc = errno(syscall.Lstat(realPath, &stgo))
			fileStat, _ := os.Stat(realPath)
			mtime := fileStat.ModTime()
			cacheStat, err := os.Stat(cachePath)

			// If file in cache is up to date return it
			if err == nil && mtime == cacheStat.ModTime() {
				// fmt.Println("cache hit!")
				stgo := syscall.Stat_t{}
				errc = errno(syscall.Lstat(cachePath, &stgo))
				copyFusestatFromGostat(stat, &stgo)
				return
			}

			executable := fmt.Sprintf("%v", exe)
			if strings.HasPrefix(executable, "./") {
				thisexe, _ := os.Executable()
				wd := filepath.Dir(thisexe)
				executable = filepath.Clean(wd + "/" + executable)
			}

			cmd := exec.Command(executable, realPath)
			cmd.Stdin = strings.NewReader("")
			var out bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr
			err = cmd.Run()
			if err == nil {
				folder := filepath.Dir(path)
				cacheFolder := filepath.Join(vfs.cache, folder)
				if _, err := os.Stat(cacheFolder); os.IsNotExist(err) {
					os.MkdirAll(cacheFolder, 0755)
				}
				ioutil.WriteFile(cachePath, out.Bytes(), 0755)
				os.Chtimes(cachePath, mtime, mtime)
				stgo := syscall.Stat_t{}
				errc = errno(syscall.Lstat(cachePath, &stgo))
				copyFusestatFromGostat(stat, &stgo)
				return
			}
			fmt.Println("Error:")
			fmt.Println(err)
			fmt.Println(stderr.String())
		}
	}

	// STANDARD PASSTHROUGH LOGIC
	stgo := syscall.Stat_t{}
	if ^uint64(0) == fh {
		path = filepath.Join(vfs.root, path)
		errc = errno(syscall.Lstat(path, &stgo))
	} else {
		errc = errno(syscall.Fstat(int(fh), &stgo))
	}
	copyFusestatFromGostat(stat, &stgo)
	return
}

func (vfs *ptfs) Read(path string, buff []byte, ofst int64, fh uint64) (n int) {
	//  defer trace(path, buff, ofst, fh)(&n)
	n, e := syscall.Pread(int(fh), buff, ofst)
	if nil != e {
		return errno(e)
	}
	return n
}

func (vfs *ptfs) Release(path string, fh uint64) (errc int) {
	//  defer trace(path, fh)(&errc)
	return errno(syscall.Close(int(fh)))
}

func (vfs *ptfs) Opendir(path string) (errc int, fh uint64) {
	//  defer trace(path)(&errc, &fh)
	path = filepath.Join(vfs.root, path)
	f, e := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if nil != e {
		return errno(e), ^uint64(0)
	}
	return 0, uint64(f)
}

func (vfs *ptfs) Readdir(path string,
	fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64,
	fh uint64) (errc int) {
	//  defer trace(path, fill, ofst, fh)(&errc)
	path = filepath.Join(vfs.root, path)
	file, e := os.Open(path)
	if nil != e {
		return errno(e)
	}
	defer file.Close()
	nams, e := file.Readdirnames(0)
	if nil != e {
		return errno(e)
	}
	nams = append([]string{".", ".."}, nams...)
	for _, name := range nams {
		if !fill(name, nil, 0) {
			break
		}
	}
	return 0
}

func (vfs *ptfs) Releasedir(path string, fh uint64) (errc int) {
	//  defer trace(path, fh)(&errc)
	return errno(syscall.Close(int(fh)))
}

func main() {

	if _, err := os.Stat(".cache"); os.IsNotExist(err) {
		// Create the folder or die.
		err := os.Mkdir(".cache", 0755)
		if err != nil {
			log.Fatal(err)
		}
	}

	if jsonFile, err := os.Open("transforms.json"); err == nil {
		defer jsonFile.Close()
		bytes, _ := ioutil.ReadAll(jsonFile)
		json.Unmarshal([]byte(bytes), &transformMap)
		// TODO: pre-cache the regexes
	}

	syscall.Umask(0)
	ptfs := ptfs{}
	args := os.Args

	ptfs.cache, _ = filepath.Abs(".cache")
	ptfs.original, _ = filepath.Abs(args[1]) // /test

	if 3 <= len(args) && '-' != args[len(args)-2][0] && '-' != args[len(args)-1][0] {
		ptfs.root, _ = filepath.Abs(args[len(args)-2])
		args = append(args[:len(args)-2], args[len(args)-1])
	}
	_host = fuse.NewFileSystemHost(&ptfs)
	_host.Mount("", args[1:])
}
