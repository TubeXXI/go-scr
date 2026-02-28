package scraper

import (
	"context"

	"github.com/chromedp/chromedp"
)

type TubeScraper struct {
	Ctx      context.Context
	cancel   context.CancelFunc
	useProxy bool
	proxyURL string
}

func NewTubeScraper() *TubeScraper {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-features", "site-per-process"),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-notifications", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.WindowSize(1920, 1080),
		chromedp.Flag("log-level", "3"), // Nonaktifkan logging Chrome
	)

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)

	ctx, cancel := chromedp.NewContext(allocCtx)

	return &TubeScraper{
		Ctx:    ctx,
		cancel: cancel,
	}
}

func (ys *TubeScraper) SetProxy(proxyURL string) {
	ys.useProxy = true
	ys.proxyURL = proxyURL
}

func (ys *TubeScraper) Close() {
	ys.cancel()
}
