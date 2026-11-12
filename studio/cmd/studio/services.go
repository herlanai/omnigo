package main

import (
	"context"
	"runtime"
	"time"

	"github.com/herlanai/omnigo"
	"github.com/herlanai/omnigo/apps/host"
	"github.com/herlanai/omnigo/apps/metrics"
	"github.com/herlanai/omnigo/studio/internal/browser"
	"github.com/herlanai/omnigo/studio/internal/catalog"
	"github.com/herlanai/omnigo/studio/internal/insights"
)

func studioRegistration(browsers *browser.Manager, monitor *metrics.Monitor) omnigo.Registration {
	return func(app *omnigo.Application) error {
		type overview struct {
			Entries  []catalog.Entry `json:"entries"`
			Native   bool            `json:"native"`
			Platform string          `json:"platform"`
		}

		type request struct {
			ID   string `json:"id"`
			Dark bool   `json:"dark"`
		}
		type reply struct {
			Opened  bool   `json:"opened"`
			Message string `json:"message"`
		}
		return omnigo.Group(
			omnigo.Query("studio.catalog", func() overview {
				return overview{catalog.Entries(), host.Available, runtime.GOOS + "/" + runtime.GOARCH}
			}),
			omnigo.Query("studio.performance", monitor.Snapshot),
			omnigo.Query("studio.icons", insights.IconCatalog),
			omnigo.Route("/studio/icon.png", insights.IconHandler()),
			omnigo.Method("studio.window.titlebar", func(ctx context.Context, r host.TitleBarRequest) (host.TitleBarState, error) {
				if r.Style == "" {
					return app.TitleBarState(ctx)
				}
				return app.SetTitleBar(ctx, r)
			}),
			omnigo.Method("studio.open", func(ctx context.Context, r request) (reply, error) {
				if !catalog.HasDemo(r.ID) {
					return reply{Message: "此项尚无可执行演示。"}, nil
				}
				if r.ID == "plugin/chromium" {
					if err := browsers.Open(ctx, r.Dark); err != nil {
						return reply{Message: err.Error()}, nil
					}
					return reply{Opened: true, Message: "已打开 Omni Browser：CEF 原生浏览器，支持标签页、地址栏和导航。"}, nil
				}
				call, stop := context.WithTimeout(ctx, 5*time.Second)
				defer stop()
				if err := app.OpenContentWindow(call, r.ID, r.Dark); err != nil {
					return reply{Message: err.Error()}, nil
				}
				return reply{Opened: true, Message: "已打开 Go 原生演示窗口。"}, nil
			}),
		)(app)
	}
}
