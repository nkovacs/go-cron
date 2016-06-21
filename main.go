package main

import (
	"github.com/namsral/flag"
	"gopkg.in/fsnotify.v1"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
)

var (
	crontabPath string
	numCPU      int
	stderr      *log.Logger
	stdout      *log.Logger
	watcher     *fsnotify.Watcher
	runner      *Runner
)

func init() {
	flag.StringVar(&crontabPath, "file", "crontab", "crontab file path")
	flag.IntVar(&numCPU, "cpu", runtime.NumCPU(), "maximum number of CPUs")
}

func parseFile(filePath string) *Runner {
	file, err := os.Open(filePath)
	if err != nil {
		stderr.Fatalf("crontab path: %v error: %v", filePath, err)
	}
	defer file.Close()

	parser, err := NewParser(file)
	if err != nil {
		stderr.Fatalf("Parser read error: %v", err)
	}
	parser.SetLogger(stdout)
	parser.SetErrorLogger(stderr)

	runner, err := parser.Parse()
	if err != nil {
		stderr.Fatalf("Parser parse error: %v", err)
	}

	return runner
}

func main() {
	flag.Parse()

	runtime.GOMAXPROCS(numCPU)

	stdout = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	stderr = log.New(os.Stderr, "", log.Ldate|log.Ltime)

	var err error
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		stderr.Fatalf("Failed to create watcher: %v", err)
	}
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					// channel closed, which can only happen when we're exiting.
					return
				}

				if event.Name != crontabPath {
					break
				}

				if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
					// removed or renamed, stop runners
					stdout.Printf("Crontab removed, stopping")
					runner.Stop()
					break
				}

				if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
					stdout.Printf("Reloading crontab")
					runner.Stop()
					runner = parseFile(crontabPath)
					runner.Start()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					// channel closed, which can only happen when we're exiting.
					return
				}
				stderr.Printf("Watcher error: %v", err)
			}
		}
	}()

	runner = parseFile(crontabPath)
	dir := filepath.Dir(crontabPath)

	var wg sync.WaitGroup
	shutdown(runner, &wg, stdout)

	runner.Start()
	wg.Add(1)

	watcher.Add(dir)

	wg.Wait()
	stdout.Println("End cron")
}

func shutdown(runner *Runner, wg *sync.WaitGroup, logger *log.Logger) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-c
		logger.Println("Got signal: ", s)
		watcher.Close()
		runner.Stop()
		wg.Done()
	}()
}
