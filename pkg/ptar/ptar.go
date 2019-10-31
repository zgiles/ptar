package ptar

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"encoding/hex"
	"fmt"

	"github.com/pierrec/lz4"
	"github.com/zgiles/ptar/pkg/index"
	// "github.com/zgiles/ptar/pkg/scanner"
	"github.com/zgiles/ptar/pkg/writecounter"

	// xz "github.com/remyoudompheng/go-liblzma"
	"io"
	"os"
	"strconv"
	"sync"
)

type Indexer interface {
	IndexWriter(io.WriteCloser)
	Close()
	Channel() chan index.IndexItem
}

type Scanner interface {
	Scan(string, chan string, chan error)
}

/*
type Partition struct {
	filename string
	entries  chan string
}
*/

type Archive struct {
	InputPath    string
	OutputPath   string
	TarThreads   int
	TarMaxSize   int
	Compression  string
	Index        bool
	Verbose      bool
	Scanner      Scanner
	Indexer      Indexer
	FileMaker    func(string) (io.WriteCloser, error)
	globalwg     *sync.WaitGroup
	scanwg       *sync.WaitGroup
	partitionswg *sync.WaitGroup
	entries      chan string
	errors       chan error
}

func NewArchive(inputpath string, outputpath string, tarthreads int, compression string, index bool) *Archive {
	arch := &Archive{
		InputPath:   inputpath,
		OutputPath:  outputpath,
		TarThreads:  tarthreads,
		Compression: compression,
		Index:       index,
	}
	// need to probably do a default scanner
	// 	ScanFunc:    scanner.Scan,
	return arch
}

func (arch *Archive) Begin() {
	arch.globalwg = new(sync.WaitGroup)
	arch.scanwg = new(sync.WaitGroup)
	arch.partitionswg = new(sync.WaitGroup)
	arch.entries = make(chan string, 1024)
	arch.errors = make(chan error, 1024)

	if arch.Scanner == nil {
		return
	}

	arch.globalwg.Add(1)
	arch.scanwg.Add(1)
	go func() {
		arch.Scanner.Scan(arch.InputPath, arch.entries, arch.errors)
		arch.scanwg.Done()
		arch.globalwg.Done()
	}()

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

func (arch *Archive) tarChannel(threadnum int) {
	defer arch.partitionswg.Done()
	defer arch.globalwg.Done()
	var r io.WriteCloser
	var ientries chan index.IndexItem
	// ientries := make(chan IndexItem, 1024)
	indexwg := new(sync.WaitGroup)

	filename := arch.OutputPath + "." + strconv.Itoa(threadnum) + ".tar" + arch.Compression
	f, ferr := arch.FileMaker(filename)
	if ferr != nil {
		panic(ferr)
	}
	// defer f.Sync()
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
		indexwg.Add(1)
		go func() {
			f, ferr := arch.FileMaker(filename + ".index")
			if ferr != nil {
				panic(ferr)
			}
			arch.Indexer.IndexWriter(f)
			indexwg.Done()
		}()
		ientries = arch.Indexer.Channel()
		defer arch.Indexer.Close()
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
	indexwg.Wait()
}
