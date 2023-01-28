package webui

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/kairos-io/kairos/internal/agent"
	"github.com/kairos-io/kairos/pkg/config"
	"github.com/labstack/echo/v4"
	process "github.com/mudler/go-processmanager"
)

type state struct {
	p *process.Process
	sync.Mutex
}

func Start(ctx context.Context) error {

	localInstallState := &state{}

	listen := config.DefaultWebUIListenAddress

	ec := echo.New()
	ec.Renderer = templateRenderer()
	agentConfig, err := agent.LoadConfig()
	if err != nil {
		return err
	}

	if agentConfig.WebUI.ListenAddress != "" {
		listen = agentConfig.WebUI.ListenAddress
	}

	if agentConfig.WebUI.Disable {
		log.Println("WebUI installer disabled by branding")
		return nil
	}

	ec.GET("/*", assetHandler())
	ec.GET("/ws", streamProcess(localInstallState))
	ec.POST("/install", installHandler(localInstallState))

	if err := ec.Start(listen); err != nil && err != http.ErrServerClosed {
		return err
	}

	go func() {
		<-ctx.Done()
		ct, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := ec.Shutdown(ct)
		if err != nil {
			log.Printf("shutdown failed: %s", err.Error())
		}
		cancel()
	}()

	return nil
}
