package ptar

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sync/atomic"

	"github.com/karrick/godirwalk"
	"github.com/pierrec/lz4"

	// xz "github.com/remyoudompheng/go-liblzma"
	"io"
	"os"
	"sync"
)

// Lib

type WriteCounter struct {
	io.Writer
	pos    uint64
	writer io.Writer
}

func NewWriteCounter(w io.Writer) *WriteCounter {
	return &WriteCounter{writer: w}
}

func (counter *WriteCounter) Write(buf []byte) (int, error) {
	n, err := counter.writer.Write(buf)
	atomic.AddUint64(&counter.pos, uint64(n))
	return n, err
}

func (counter *WriteCounter) Pos() uint64 {
	return atomic.LoadUint64(&counter.pos)
}

// End Lib

type Partition struct {
	filename string
	entries  chan string
}

type IndexItem struct {
	hash string
	pos  uint64
	size uint64
	name string
}

func Scan(wg *sync.WaitGroup, path string, entries chan string, e chan error) {
	err := godirwalk.Walk(path, &godirwalk.Options{
		Unsorted: true,
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			entries <- osPathname
			return nil
		},
		ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
			e <- err
			return godirwalk.SkipNode
		},
	})
	if err != nil {
		e <- err
	}
	close(entries)
	wg.Done()
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

func Errornotice(wg *sync.WaitGroup, e chan error) {
	for {
		err, ok := <-e
		if !ok {
			break
		} else {
			if err != nil {
				fmt.Printf("ERROR: %s\n", err)
			}
		}
	}
	wg.Done()
}

func indexwriter(wg *sync.WaitGroup, file string, entries chan IndexItem) {
	defer wg.Done()
	f, ferr := os.Create(file)
	if ferr != nil {
		panic(ferr)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for {
		i, ok := <-entries
		if !ok {
			break
		}
		if i.hash == "" {
			fmt.Fprintf(w, "%d:%d::%s\n", i.pos, i.size, i.name)
		} else {
			fmt.Fprintf(w, "%d:%d:%16s:%s\n", i.pos, i.size, i.hash, i.name)
		}
		w.Flush()
	}
}

func TarChannel(wg *sync.WaitGroup, file string, entries chan string, e chan error, compress string, index bool, verbose bool) {
	defer wg.Done()
	var r io.WriteCloser
	ientries := make(chan IndexItem, 1024)
	indexwg := new(sync.WaitGroup)

	f, ferr := os.Create(file)
	if ferr != nil {
		panic(ferr)
	}
	defer f.Sync()
	defer f.Close()

	switch compress {
	/*
		case "xz":
			var err error
			r, err = xz.NewWriter(f)
			if err != nil {
				panic(err)
			}
	*/
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

	if index {
		indexwg.Add(1)
		go indexwriter(indexwg, file+".ptarindex", ientries)
	}

	cw := NewWriteCounter(r)
	tw := tar.NewWriter(cw)
	defer tw.Close()

	for {
		i, ok := <-entries
		if !ok {
			break
		}
		if verbose {
			fmt.Printf("%s\n", i)
		}
		s, serr := os.Lstat(i)
		if serr != nil {
			e <- serr
			panic(serr)
		}

		var ientry IndexItem
		if index {
			ientry = IndexItem{name: i}
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

		if index {
			ientry.pos = cw.Pos()
		}

		if err := tw.WriteHeader(hdr); err != nil {
			e <- err
			panic(err)
		}

		// Only call Write if it's a regular file; all others are invalid
		if hdr.Typeflag == tar.TypeReg {
			hash := sha1.New()
			sf, sferr := os.Open(i)
			if sferr != nil {
				e <- sferr
				panic(sferr)
			}
			b := make([]byte, 4096)
			for {
				c, err := sf.Read(b)
				if err != nil && err != io.EOF {
					e <- err
					panic(err)
				}
				if c == 0 {
					break
				}
				if index {
					hash.Write(b[:c])
				}
				if _, err := tw.Write(b[:c]); err != nil {
					e <- err
					panic(err)
				}
			}
			closeerr := sf.Close()
			if closeerr != nil {
				panic(closeerr)
			}
			if index {
				ientry.hash = hex.EncodeToString(hash.Sum(nil))
			}
		}
		tw.Flush()

		if index {
			ientry.size = cw.Pos() - ientry.pos
			ientries <- ientry
			// log.Printf("%v", ientry)
		}
	}
	// log.Printf("Pre Write Pos: %d\n", cw.Pos())
	// log.Printf("Closing Write Pos: %d\n", cw.Pos())
	close(ientries)
	indexwg.Wait()
}
