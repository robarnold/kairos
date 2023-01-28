package webui

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	process "github.com/mudler/go-processmanager"
	"github.com/nxadm/tail"
	"golang.org/x/net/websocket"
)

type FormData struct {
	CloudConfig string `form:"cloud-config" json:"cloud-config" query:"cloud-config"`
	Reboot      string `form:"reboot" json:"reboot" query:"reboot"`

	PowerOff           string `form:"power-off" json:"power-off" query:"power-off"`
	InstallationDevice string `form:"installation-device" json:"installation-device" query:"installation-device"`
}

func installHandler(localInstallState *state) func(c echo.Context) error {
	return func(c echo.Context) error {

		localInstallState.Lock()
		if localInstallState.p != nil {
			status, _ := localInstallState.p.ExitCode()
			if localInstallState.p.IsAlive() || status == "0" {
				localInstallState.Unlock()
				return c.Redirect(http.StatusSeeOther, "progress.html")
			}
		}
		localInstallState.Unlock()

		formData := new(FormData)
		if err := c.Bind(formData); err != nil {
			return err
		}

		// Process the form data as necessary
		cloudConfig := formData.CloudConfig
		reboot := formData.Reboot
		powerOff := formData.PowerOff
		installationDevice := formData.InstallationDevice

		args := []string{"manual-install"}

		if powerOff == "on" {
			args = append(args, "--poweroff")
		}
		if reboot == "on" {
			args = append(args, "--reboot")
		}
		args = append(args, "--device", installationDevice)

		// create tempfile to store cloud-config, bail out if we fail as we couldn't go much further
		file, err := ioutil.TempFile("", "install-webui")
		if err != nil {
			log.Fatalf("could not create tmpfile for cloud-config: %s", err.Error())
		}

		err = os.WriteFile(file.Name(), []byte(cloudConfig), 0600)
		if err != nil {
			log.Fatalf("could not write tmpfile for cloud-config: %s", err.Error())
		}

		args = append(args, file.Name())

		localInstallState.Lock()
		localInstallState.p = process.New(process.WithName("/usr/bin/kairos-agent"), process.WithArgs(args...), process.WithTemporaryStateDir())
		localInstallState.Unlock()
		err = localInstallState.p.Run()
		if err != nil {
			return c.Render(http.StatusOK, "message.html", map[string]interface{}{
				"message": err.Error(),
				"type":    "danger",
			})
		}

		// Start install process, lock with sentinel
		return c.Redirect(http.StatusSeeOther, "progress.html")
	}

}

func streamProcess(s *state) func(c echo.Context) error {
	return func(c echo.Context) error {
		consumeError := func(err error) {
			if err != nil {
				c.Logger().Error(err)
			}
		}
		websocket.Handler(func(ws *websocket.Conn) {
			defer ws.Close()
			for {
				s.Lock()
				if s.p == nil {
					// Write
					err := websocket.Message.Send(ws, "No process!")
					consumeError(err)
					s.Unlock()
					return
				}
				s.Unlock()

				if !s.p.IsAlive() {
					errOut, err := os.ReadFile(s.p.StderrPath())
					if err == nil {
						err := websocket.Message.Send(ws, string(errOut))
						consumeError(err)
					}
					out, err := os.ReadFile(s.p.StdoutPath())
					if err == nil {
						err = websocket.Message.Send(ws, string(out))
						consumeError(err)
					}
					err = websocket.Message.Send(ws, "Process stopped!")
					consumeError(err)
					return
				}

				t, err := tail.TailFile(s.p.StdoutPath(), tail.Config{Follow: true})
				if err != nil {
					return
				}
				t2, err := tail.TailFile(s.p.StderrPath(), tail.Config{Follow: true})
				if err != nil {
					return
				}

				for {
					select {
					case line := <-t.Lines:
						err = websocket.Message.Send(ws, line.Text+"\r\n")
						consumeError(err)
					case line := <-t2.Lines:
						err = websocket.Message.Send(ws, line.Text+"\r\n")
						consumeError(err)
					}
				}
			}
		}).ServeHTTP(c.Response(), c.Request())
		return nil
	}
}
