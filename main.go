package main

import (
	"fmt"
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

func parseFile(filePath string) (*Runner, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("crontab path: %v error: %v", filePath, err)
	}
	defer file.Close()

	parser, err := NewParser(file)
	if err != nil {
		return nil, fmt.Errorf("Parser read error: %v", err)
	}
	parser.SetLogger(stdout)
	parser.SetErrorLogger(stderr)

	runner, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("Parser parse error: %v", err)
	}

	return runner, nil
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
					if runner != nil {
						runner.Stop()
					}
					break
				}

				if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
					stdout.Printf("Reloading crontab")
					if runner != nil {
						runner.Stop()
					}
					var err error
					runner, err = parseFile(crontabPath)
					if err != nil {
						stderr.Print(err.Error())
						break
					}
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

	runner, err = parseFile(crontabPath)
	if err != nil {
		stderr.Print(err.Error())
	}
	dir := filepath.Dir(crontabPath)

	if runner != nil {
		runner.Start()
	}

	var wg sync.WaitGroup
	shutdown(runner, &wg, stdout)
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
		if runner != nil {
			runner.Stop()
		}
		wg.Done()
	}()
}
