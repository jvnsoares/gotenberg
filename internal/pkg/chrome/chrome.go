package chrome

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/mafredri/cdp/devtool"
	"github.com/thecodingmachine/gotenberg/internal/pkg/xerror"
	"github.com/thecodingmachine/gotenberg/internal/pkg/xexec"
	"github.com/thecodingmachine/gotenberg/internal/pkg/xlog"
	"github.com/thecodingmachine/gotenberg/internal/pkg/xtime"
)

// Start starts Google Chrome headless in background.
func Start(logger xlog.Logger, ignoreCertificateErrors bool) error {
	const op string = "chrome.Start"
	logger.DebugOp(op, "starting new Google Chrome headless process on port 9222...")
	resolver := func() error {
		cmd, err := cmd(logger, ignoreCertificateErrors)
		if err != nil {
			return err
		}
		// we try to start the process.
		xexec.LogBeforeExecute(logger, cmd)
		if err := cmd.Start(); err != nil {
			return err
		}
		// if the process failed to start correctly,
		// we have to restart it.
		if !isViable(logger) {
			return restart(logger, cmd.Process, ignoreCertificateErrors)
		}
		return nil
	}
	if err := resolver(); err != nil {
		return xerror.New(op, err)
	}
	return nil
}

func cmd(logger xlog.Logger, ignoreCertificateErrors bool) (*exec.Cmd, error) {
	const op string = "chrome.cmd"
	binary := "google-chrome-stable"
	args := []string{
		"--no-sandbox",
		"--headless",
		// see https://github.com/thecodingmachine/gotenberg/issues/157.
		"--disable-dev-shm-usage",
		// See https://github.com/puppeteer/puppeteer/issues/661
		// and https://github.com/puppeteer/puppeteer/issues/2410.
		"--font-render-hinting=none",
		"--remote-debugging-port=9222",
		"--disable-gpu",
		"--disable-translate",
		"--disable-extensions",
		"--disable-background-networking",
		"--safebrowsing-disable-auto-update",
		"--disable-sync",
		"--disable-default-apps",
		"--hide-scrollbars",
		"--metrics-recording-only",
		"--mute-audio",
		"--no-first-run",
	}

	if ignoreCertificateErrors {
		args = append(args, "--ignore-certificate-errors")
	}

	cmd, err := xexec.Command(logger, binary, args...)
	if err != nil {
		return nil, xerror.New(op, err)
	}
	// cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd, nil
}

func kill(logger xlog.Logger, proc *os.Process) error {
	const op string = "chrome.kill"
	logger.DebugOp(op, "killing Google Chrome headless process using port 9222...")
	resolver := func() error {
		// err := syscall.Kill(-proc.Pid, syscall.SIGKILL)
		kill := exec.Command("TASKKILL", "/T", "/F", "/PID", strconv.Itoa(proc.Pid))
		er := kill.Run()
		if er == nil {
			return nil
		}
		if strings.Contains(er.Error(), "no such process") {
			return nil
		}
		return er
	}
	if err := resolver(); err != nil {
		return xerror.New(op, err)
	}
	return nil
}

func restart(logger xlog.Logger, proc *os.Process, ignoreCertificateErrors bool) error {
	const op string = "chrome.restart"
	logger.DebugOp(op, "restarting Google Chrome headless process using port 9222...")
	resolver := func() error {
		// kill the existing process first.
		if err := kill(logger, proc); err != nil {
			return err
		}
		cmd, err := cmd(logger, ignoreCertificateErrors)
		if err != nil {
			return err
		}
		// we try to restart the process.
		xexec.LogBeforeExecute(logger, cmd)
		if err := cmd.Start(); err != nil {
			return err
		}
		// if the process failed to restart correctly,
		// we have to restart it again.
		if !isViable(logger) {
			return restart(logger, cmd.Process, ignoreCertificateErrors)
		}
		return nil
	}
	if err := resolver(); err != nil {
		return xerror.New(op, err)
	}
	return nil
}

func isViable(logger xlog.Logger) bool {
	const (
		op                string = "chrome.isViable"
		maxViabilityTests int    = 20
	)
	viable := func() bool {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		endpoint := "http://localhost:9222"
		logger.DebugOpf(
			op,
			"checking Google Chrome headless process viability via endpoint '%s/json/version'",
			endpoint,
		)
		v, err := devtool.New(endpoint).Version(ctx)
		if err != nil {
			logger.DebugOpf(
				op,
				"Google Chrome headless is not viable as endpoint returned '%v'",
				err.Error(),
			)
			return false
		}
		logger.DebugOpf(
			op,
			"Google Chrome headless is viable as endpoint returned '%v'",
			v,
		)
		return true
	}
	result := false
	for i := 0; i < maxViabilityTests && !result; i++ {
		warmup(logger)
		result = viable()
	}
	return result
}

func warmup(logger xlog.Logger) {
	const (
		op      string  = "chrome.warmup"
		seconds float64 = 0.5
	)
	warmupTime := xtime.Duration(seconds)
	logger.DebugOpf(
		op,
		"waiting '%v' for allowing Google Chrome to warmup",
		warmupTime,
	)
	time.Sleep(warmupTime)
}
