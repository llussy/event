package renderer

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lodastack/event/config"
	"github.com/lodastack/log"
)

const (
	PhantomjsBin = "phantomjs"
	RenderScript = "render.js"
)

type RenderOps struct {
	ID          string
	Ns          string
	Measurement string
	Time        time.Time
	Fn          string
	Title       string
	Where       string
	// Width  string
	// Height string
	// Timeout string
}

func RenderURL(params RenderOps) string {
	return fmt.Sprintf("%s?ns=%s&measurement=%s&starttime=%d&endtime=%d&fn=%s&title=%s&where=%s",
		config.GetConfig().Render.RenderURL, params.Ns, params.Measurement,
		params.Time.Add(-60*time.Minute).Unix()*1000, params.Time.Unix()*1000,
		params.Fn, params.Title, params.Where)
}

func RenderToPng(params RenderOps) (string, error) {
	binPath, err := filepath.Abs(filepath.Join(config.GetConfig().Render.PhantomDir, PhantomjsBin))
	if err != nil {
		return "", err
	}
	renderScript, err := filepath.Abs(filepath.Join(config.GetConfig().Render.PhantomDir, RenderScript))
	if err != nil {
		return "", err
	}

	replaceLetterFunc := func(r rune) rune {
		if r == '"' || r == '\'' || r == '/' || r == '(' || r == ')' {
			return '_'
		}
		return r
	}
	filename := strings.Map(replaceLetterFunc, params.ID+".png")
	pngPath, err := filepath.Abs(filepath.Join(config.GetConfig().Render.ImgDir, filename))
	if err != nil {
		return "", err
	}

	renderURL := RenderURL(params)
	cmdArgs := []string{
		"--ignore-ssl-errors=true",
		"--proxy-type=none",
		renderScript,
		"png=" + pngPath,
		"url=" + renderURL,
		"width=1000",
		"height=500",
	}

	cmd := exec.Command(binPath, cmdArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error("RenderToPng fail:", err.Error())
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Error("RenderToPng fail:", err.Error())
		return "", err
	}
	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stdout, stderr)

	done := make(chan error)
	err = cmd.Start()
	if err != nil {
		log.Error("start RenderToPng fail")
		return "", err
	}
	go func() {
		defer close(done)
		err := cmd.Wait()
		if err != nil {
			done <- err
		}
	}()

	timeout := 15 // TODO
	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		log.Errorf("renderToPng timeout (>%ds)", timeout)
		if err := cmd.Process.Kill(); err != nil {
			log.Error("failed to kill", "error", err)
		}
		return "", fmt.Errorf("renderToPng timeout (>%ds)", timeout)
	case err := <-done:
		if err != nil {
			log.Errorf("renderToPng fail: %s", err)
			return "", err
		}
	}

	log.Debug("Image rendered", "path", pngPath)
	return pngPath, nil
}
