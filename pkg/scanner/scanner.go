package scanner

import (
	"github.com/karrick/godirwalk"
)

/*
type IndexItem struct {
	hash string
	pos  uint64
	size uint64
	name string
}
*/

func Scan(inputpath string, entries chan string, errors chan error) {
	err := godirwalk.Walk(inputpath, &godirwalk.Options{
		Unsorted: true,
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			entries <- osPathname
			return nil
		},
		ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
			errors <- err
			return godirwalk.SkipNode
		},
	})
	if err != nil {
		errors <- err
	}
	close(entries)
	// arch.scanwg.Done()
	// arch.globalwg.Done()
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
*/
