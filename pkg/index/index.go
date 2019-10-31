package index

import (
	"bufio"
	"fmt"
	"os"
	// "sync"
)

type Index struct {
	filename string
	c        chan IndexItem
	//	wg       *sync.WaitGroup
}

type IndexItem struct {
	Hash string
	Pos  uint64
	Size uint64
	Name string
}

func NewIndex() *Index {
	index := &Index{}
	// index.filename = filename
	// index.wg = &sync.WaitGroup{}
	index.c = make(chan IndexItem)
	// index.wg.Add(1)
	// go index.IndexWriter()
	return index
}

func (index *Index) IndexWriter(filename string) {
	index.filename = filename
	// defer index.wg.Done()
	var f *os.File
	f, ferr := os.Create(index.filename)
	if ferr != nil {
		panic(ferr)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	for {
		i, ok := <-index.c
		if !ok {
			break
		}
		if i.Hash == "" {
			fmt.Fprintf(w, "%d:%d::%s\n", i.Pos, i.Size, i.Name)
			fmt.Printf("%d:%d::%s\n", i.Pos, i.Size, i.Name)
		} else {
			fmt.Fprintf(w, "%d:%d:%16s:%s\n", i.Pos, i.Size, i.Hash, i.Name)
			fmt.Printf("%d:%d:%16s:%s\n", i.Pos, i.Size, i.Hash, i.Name)
		}
		// w.Flush()
	}
}

/*
func (index *Index) Wait() {
	index.wg.Wait()
}
*/

func (index *Index) Close() {
	close(index.c)
}

func (index *Index) Channel() chan IndexItem {
	return index.c
}

/*
func LineWriterChannel(index.wg *sync.WaitGroup, entries chan string, e chan error, verbose bool) {
	defer index.wg.Done()
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
