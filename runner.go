package main

import (
	"github.com/nkovacs/cron"
	"log"
	"os"
	"os/exec"
)

type Runner struct {
	cron        *cron.Cron
	logger      *log.Logger
	errorLogger *log.Logger
}

func NewRunner() *Runner {
	r := &Runner{
		cron:        cron.New(),
		logger:      log.New(os.Stdout, "", log.LstdFlags),
		errorLogger: log.New(os.Stderr, "", log.LstdFlags),
	}
	return r
}

func (r *Runner) SetLogger(logger *log.Logger) {
	r.logger = logger
}

func (r *Runner) SetErrorLogger(errorLogger *log.Logger) {
	r.errorLogger = errorLogger
}

func (r *Runner) Add(spec string, cmd string) error {
	r.logger.Printf("Add cron job spec:%v cmd:%v", spec, cmd)

	err := r.cron.AddFunc("0 "+spec, r.cmdFunc(cmd))
	if err != nil {
		r.errorLogger.Printf("Error adding cron job spec: %v cmd: %v err: %v", spec, cmd, err)
	}

	return err
}

func (r *Runner) Len() int {
	return len(r.cron.Entries())
}

func (r *Runner) Start() {
	r.logger.Println("Start runner")
	r.cron.Start()
}

func (r *Runner) Stop() {
	r.cron.Stop()
	r.logger.Println("Stop runner")
}

func (r *Runner) cmdFunc(cmd string) func() {
	cmdFunc := func() {
		r.logger.Printf("cmd: %v", cmd)
		out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
		r.logger.Printf("output:\n%s", out)
		if err != nil {
			r.errorLogger.Printf("cmd: %v, err: %v", cmd, err)
		}
	}
	return cmdFunc
}
