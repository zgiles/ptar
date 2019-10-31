package main

import (
	"gopkg.in/alecthomas/kingpin.v2"
	"io"
	"log"
	"os"

	"github.com/zgiles/ptar/pkg/index"
	"github.com/zgiles/ptar/pkg/ptar"
	"github.com/zgiles/ptar/pkg/scanner"
)

type rootConfig struct {
	Compression string
	Prefix      string
	Input       string
	Debug       bool
	Index       bool
	Threads     int
	Verbose     bool
	GOGCPercent int
	GOMAXProcs  int
	Create      bool
}

func RegularFileCreate(filename string) (io.WriteCloser, error) {
	return os.Create(filename)
}

var version string
var config rootConfig
var logger *log.Logger

func main() {
	/*
		Example Desired end state
		./ptar \
		--partition dirdepth2,unixgroup \
		--input /things \
		--output ./files/ \
		--compression gzip \
		--parallel 2 \
		--maxoutputsize 5TB \
		--manifest yes
	*/

	config = rootConfig{}

	app := kingpin.New("ptar", "Parallel Tar")
	app.UsageTemplate(kingpin.CompactUsageTemplate)
	app.Flag("create", "Create").Short('c').BoolVar(&config.Create)
	app.Flag("threads", "Threads").Short('t').Default("16").IntVar(&config.Threads)
	app.Flag("debug", "Enable debug output").BoolVar(&config.Debug)
	app.Flag("verbose", "Verbose Mode").Short('v').BoolVar(&config.Verbose)
	app.Flag("gogcpercent", "GO GC Percent").Default("0").IntVar(&config.GOGCPercent)
	app.Flag("gomaxprocs", "GO Max Procs").Default("0").IntVar(&config.GOMAXProcs)
	app.Flag("compression", "Compression type").HintOptions("gzip", "lz4").StringVar(&config.Compression)
	app.Flag("file", "(File) Prefix to use for output files. Ex: output => output.tar.gz").Required().Short('f').StringVar(&config.Prefix)
	app.Flag("index", "Enable Index output").BoolVar(&config.Index)
	app.Arg("input", "Input Path(s)").Required().StringVar(&config.Input)
	app.Version(version)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	if config.Debug {
		log.Println("Config:")
		log.Println("  Parallel: ", config.Threads)
		log.Println("  Compression: ", config.Compression)
		log.Println("  Prefix: ", config.Prefix)
		log.Println("  Input: ", config.Input)
		log.Println("  Debug: ", config.Debug)
		log.Println("  Indexes: ", config.Index)
	}

	// NewArchive(inputpath string, outputpath string, tarthreads int, compression string, index bool) (*Archive)
	arch := ptar.NewArchive(config.Input, config.Prefix, config.Threads, config.Compression, config.Index)
	arch.Verbose = config.Verbose
	arch.Scanner = scanner.NewScanner()
	arch.Indexer = index.NewIndex
	arch.FileMaker = RegularFileCreate
	arch.Begin()
}
