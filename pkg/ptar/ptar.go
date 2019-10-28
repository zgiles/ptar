package ptar

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	// "syscall"

	"github.com/karrick/godirwalk"
	"github.com/pierrec/lz4"
	"github.com/zgiles/ptar/pkg/index"
	"github.com/zgiles/ptar/pkg/writecounter"

	// xz "github.com/remyoudompheng/go-liblzma"
	"io"
	"os"
	"sync"
	"strconv"
)

type Partition struct {
	filename string
	entries  chan string
}

/*
type IndexItem struct {
	hash string
	pos  uint64
	size uint64
	name string
}
*/

type Archive struct {
	InputPath	string
	OutputPath	string
	TarThreads	int
	TarMaxSize	int
	Compression	string
	Index		bool
	Verbose		bool
	globalwg *sync.WaitGroup
	scanwg *sync.WaitGroup
	partitionswg *sync.WaitGroup
	entries chan string
	errors chan error
}

func NewArchive(inputpath string, outputpath string, tarthreads int, compression string, index bool) (*Archive) {
	arch := &Archive{
		InputPath: inputpath,
		OutputPath: outputpath,
		TarThreads: tarthreads,
		Compression: compression,
		Index: index,
	}
	return arch
}

func (arch *Archive) Begin() {
	arch.globalwg = new(sync.WaitGroup)
	arch.scanwg = new(sync.WaitGroup)
	arch.partitionswg = new(sync.WaitGroup)
	arch.entries = make(chan string, 1024)
	arch.errors = make(chan error, 1024)

	arch.globalwg.Add(1)
	arch.scanwg.Add(1)
	go arch.scan()

	arch.globalwg.Add(1)
	go arch.errornotice()

	// for all partitions
	// arch.globalwg.Add(1)
	for i := 0; i < arch.TarThreads; i++ {
		arch.partitionswg.Add(1)
		arch.globalwg.Add(1)
		go arch.tarChannel(i)
	}
	// go channelcounter(wg, "files", entries)
	arch.scanwg.Wait()
	// arch.globalwg.Done()
	arch.partitionswg.Wait()

	close(arch.errors)
	arch.globalwg.Wait()
}

func (arch *Archive) scan() {
	err := godirwalk.Walk(arch.InputPath, &godirwalk.Options{
		Unsorted: true,
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			arch.entries <- osPathname
			return nil
		},
		ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
			arch.errors <- err
			return godirwalk.SkipNode
		},
	})
	if err != nil {
		arch.errors <- err
	}
	close(arch.entries)
	arch.scanwg.Done()
	arch.globalwg.Done()
}

func channelcounter(wg *sync.WaitGroup, t string, c chan string) {
	counter := 0
	for {
		_, ok := <-c
		if !ok {
			break
		} else {
			counter++
		}
	}
	fmt.Printf("number of %s: %d\n", t, counter)
	wg.Done()
}

func (arch *Archive) errornotice() {
	for {
		err, ok := <-arch.errors
		if !ok {
			break
		} else {
			if err != nil {
				fmt.Printf("ERROR: %s\n", err)
			}
		}
	}
	arch.globalwg.Done()
}

/*
func indexwriter(wg *sync.WaitGroup, file string, entries chan index.IndexItem) {
	defer wg.Done()
	var f *os.File
	f, ferr := os.Create(file)
	if ferr != nil {
		panic(ferr)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	for {
		i, ok := <-entries
		if !ok {
			break
		}
		if i.hash == "" {
			fmt.Fprintf(w, "%d:%d::%s\n", i.pos, i.size, i.name)
			fmt.Printf("%d:%d::%s\n", i.pos, i.size, i.name)
		} else {
			fmt.Fprintf(w, "%d:%d:%16s:%s\n", i.pos, i.size, i.hash, i.name)
			fmt.Printf("%d:%d:%16s:%s\n", i.pos, i.size, i.hash, i.name)
		}
		// w.Flush()
	}
}
*/

/*
func LineWriterChannel(wg *sync.WaitGroup, entries chan string, e chan error, verbose bool) {
	defer wg.Done()
	for {
		i, ok := <-entries
		if !ok {
			break
		}
		s, serr := os.Lstat(i)
		fi, fiok := s.Sys().(*syscall.Stat_t)
		if !fiok {
			panic("syscall.Stat_t type assertion failed")
		}
		// inode,parent-inode,directory-depth,"filename","fileExtension",UID,GID,
		// st_size,st_dev,st_blocks,st_nlink,"st_mode",st_atime,st_mtime,st_ctime,pw_fcount,pw_dirsum
		fmt.Printf("%d,%d,0,\"%s\",\"\",%d,%d,%d,%d,%d,%d,%d,%d,%d,%d\n", fi.Ino, 0, i, fi.Uid, fi.Gid, fi.Size, fi.Dev, fi.Blocks, fi.Nlink, fi.Mode, fi.Atim, fi.Mtim, fi.Ctim)
		if serr != nil {
			e <- serr
			panic(serr)
		}
	}

}
*/


func (arch *Archive) tarChannel(threadnum int) {
	defer arch.partitionswg.Done()
	defer arch.globalwg.Done()
	var r io.WriteCloser
	var ientries chan index.IndexItem
	// ientries := make(chan IndexItem, 1024)
	// indexwg := new(sync.WaitGroup)

	filename := arch.OutputPath+"."+strconv.Itoa(threadnum)+".tar"+arch.Compression
	f, ferr := os.Create(filename)
	if ferr != nil {
		panic(ferr)
	}
	defer f.Sync()
	defer f.Close()

	switch arch.Compression {
	case "gz":
		var err error
		r = gzip.NewWriter(f)
		if err != nil {
			panic(err)
		}
		defer r.Close()
	case "lz4":
		r = lz4.NewWriter(f)
		defer r.Close()
	case "":
		r = f
	default:
		panic("Not implemented")
	}

	if arch.Index {
		tarindex := index.NewIndex(filename+".index")
		ientries = tarindex.C
		defer tarindex.Close()
		// indexwg.Add(1)
		// go indexwriter(indexwg, filename+".index", ientries)
	}

	cw := writecounter.NewWriteCounter(r)
	tw := tar.NewWriter(cw)
	defer tw.Close()

	for {
		i, ok := <-arch.entries
		if !ok {
			break
		}
		if arch.Verbose {
			fmt.Printf("%s\n", i)
		}
		s, serr := os.Lstat(i)
		if serr != nil {
			arch.errors <- serr
			panic(serr)
		}

		var ientry index.IndexItem
		if arch.Index {
			ientry = index.IndexItem{Name: i}
		}

		var link string
		var linkerr error
		if s.Mode()&os.ModeSymlink != 0 {
			link, linkerr = os.Readlink(i)
			if linkerr != nil {
				panic(linkerr)
			}
		}

		hdr, err := tar.FileInfoHeader(s, link)
		if err != nil {
			panic(linkerr)
		}
		hdr.Name = i
		hdr.Format = tar.FormatGNU
		if hdr.Typeflag == tar.TypeDir {
			hdr.Name += "/"
		}

		if arch.Index {
			ientry.Pos = cw.Pos()
		}

		if err := tw.WriteHeader(hdr); err != nil {
			arch.errors <- err
			panic(err)
		}

		// Only call Write if it's a regular file; all others are invalid
		if hdr.Typeflag == tar.TypeReg {
			hash := sha1.New()
			sf, sferr := os.Open(i)
			if sferr != nil {
				arch.errors <- sferr
				panic(sferr)
			}
			b := make([]byte, 4096)
			for {
				c, err := sf.Read(b)
				if err != nil && err != io.EOF {
					arch.errors <- err
					panic(err)
				}
				if c == 0 {
					break
				}
				if arch.Index {
					hash.Write(b[:c])
				}
				if _, err := tw.Write(b[:c]); err != nil {
					arch.errors <- err
					panic(err)
				}
			}
			closeerr := sf.Close()
			if closeerr != nil {
				panic(closeerr)
			}
			if arch.Index {
				ientry.Hash = hex.EncodeToString(hash.Sum(nil))
			}
		}
		tw.Flush()

		if arch.Index {
			ientry.Size = cw.Pos() - ientry.Pos
			ientries <- ientry
			// log.Printf("%v", ientry)
		}
	}
	// fmt.Printf("Pre Write Pos: %d\n", cw.Pos())
	// fmt.Printf("Closing Write Pos: %d\n", cw.Pos())
	// close(ientries)
	// indexwg.Wait()
}
