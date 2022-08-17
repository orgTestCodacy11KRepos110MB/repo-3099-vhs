package dolly

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// Dolly is the object that controls the setup.
type Dolly struct {
	Options *DollyOptions
	Page    *rod.Page
	Start   func()
	Cleanup func()
}

// DollyOptions is the set of options for the setup.
type DollyOptions struct {
	Framerate float64
	Height    int
	Padding   string
	Width     int

	TTY TTYOptions
	GIF GIFOptions
}

// DefaultDollyOptions returns the default set of options to use for the setup function.
func DefaultDollyOptions() DollyOptions {
	return DollyOptions{
		Framerate: 60,
		Height:    600,
		Width:     1200,
		Padding:   "5em",

		TTY: DefaultTTYOptions,
		GIF: DefaultGIFOptions,
	}
}

// New sets up ttyd and go-rod for recording frames.
func New() Dolly {
	port := randomPort()
	tty := StartTTY(port)
	go tty.Run()

	browser := rod.New().MustConnect()
	page := browser.MustPage(fmt.Sprintf("http://localhost:%d", port))

	opts := DefaultDollyOptions()

	return Dolly{
		Options: &opts,
		Page:    page,
		Start: func() {
			os.MkdirAll(opts.GIF.InputFolder, os.ModePerm)

			page = page.MustSetViewport(opts.Width, opts.Height, 1, false)
			page = page.MustWaitLoad()
			page = page.MustWaitIdle()

			page.Eval(fmt.Sprintf("term.setOption('fontFamily', '%s')", opts.TTY.FontFamily))
			page.Eval(fmt.Sprintf("term.setOption('fontSize', '%d')", opts.TTY.FontSize))
			page.Eval(fmt.Sprintf("term.setOption('lineHeight', '%f')", opts.TTY.LineHeight))
			theme, _ := json.Marshal(opts.TTY.Theme)
			page.Eval(fmt.Sprintf("term.setOption('theme', '%s')", string(theme)))

			page.MustElement(".xterm").Eval(fmt.Sprintf(`this.style.padding = '%s'`, opts.Padding))
			page.MustElement("body").Eval(`this.style.overflow = 'hidden'`)
			page.MustElement("#terminal-container").Eval(`this.style.overflow = 'hidden'`)
			page.MustElement(".xterm-viewport").Eval(`this.style.overflow = 'hidden'`)
			// Fit ttyd xterm window to the screen.
			// ttyd stores its xterm instance at `window.term`
			// https://xtermjs.org/docs/api/addons/fit/
			// https://github.com/tsl0922/ttyd/blob/723ae966939527e8db35f27fb69bac0e02860099/html/src/components/terminal/index.tsx#L167-L196
			page.MustEval("window.term.fit")
			page.MustElement("textarea").MustInput("PROMPT='%F{#5a56e0}>%f '").MustType(input.Enter)
			page.MustElement("textarea").MustInput("clear").MustType(input.Enter)
			page.MustWaitIdle()

			time.Sleep(2 * time.Second)
			go func() {
				counter := 0
				for {
					counter++
					if page != nil {
						screenshot, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
						if err != nil {
							time.Sleep(time.Second / time.Duration(opts.Framerate))
							continue
						}
						os.WriteFile((opts.GIF.InputFolder + "/" + fmt.Sprintf(frameFileFormat, counter)), screenshot, 0644)
					}
					time.Sleep(time.Second / time.Duration(opts.Framerate))
				}
			}()
		},
		Cleanup: func() {
			// Tear down the processes we started.
			browser.MustClose()
			tty.Process.Kill()

			// Make GIF with frames
			err := MakeGIF(opts.GIF).Run()

			// Cleanup frames if we successfully made the GIF.
			if err == nil {
				os.RemoveAll(opts.GIF.InputFolder)
			}
		},
	}
}