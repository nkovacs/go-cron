package main

import (
	"github.com/namsral/flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
)

var (
	crontabPath string
	numCPU      int
	stderr      *log.Logger
	stdout      *log.Logger
)

func init() {
	flag.StringVar(&crontabPath, "file", "crontab", "crontab file path")
	flag.IntVar(&numCPU, "cpu", runtime.NumCPU(), "maximum number of CPUs")
}

func parseFile(filePath string) *Runner {
	file, err := os.Open(filePath)
	if err != nil {
		stderr.Fatalf("crontab path:%v err:%v", filePath, err)
	}
	defer file.Close()

	parser, err := NewParser(file)
	if err != nil {
		stderr.Fatalf("Parser read err:%v", err)
	}
	parser.SetLogger(stdout)
	parser.SetErrorLogger(stderr)

	runner, err := parser.Parse()
	if err != nil {
		stderr.Fatalf("Parser parse err:%v", err)
	}

	return runner
}

func main() {
	flag.Parse()

	runtime.GOMAXPROCS(numCPU)

	stdout = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	stderr = log.New(os.Stderr, "", log.Ldate|log.Ltime)

	runner := parseFile(crontabPath)

	var wg sync.WaitGroup
	shutdown(runner, &wg, stdout)

	runner.Start()
	wg.Add(1)

	wg.Wait()
	stdout.Println("End cron")
}

func shutdown(runner *Runner, wg *sync.WaitGroup, logger *log.Logger) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-c
		logger.Println("Got signal: ", s)
		runner.Stop()
		wg.Done()
	}()
}
