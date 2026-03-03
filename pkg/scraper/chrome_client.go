package scraper

import (
	"context"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"
	"tubexxi/scraper/pkg/logger"

	"github.com/chromedp/chromedp"
	"github.com/sony/gobreaker/v2"
	"go.uber.org/zap"
)

const (
	MovieBaseURL  = "https://tv8.lk21official.cc/"
	SeriesBaseURL = "https://tv3.nontondrama.my/"
	AnimeBaseURL  = "https://otakudesu.best/"
	USE_XUI_PROXY = true
	XUI_HOST      = "x-ui.agcforge.com"
	XUI_USERNAME  = "tubexxi"
	XUI_PASSWORD  = "ngfbMybEOf"
)

var (
	httpPorts  = []int{8080, 8081, 8082}
	socksPorts = []int{1080, 1081, 1082}

	httpPortIndex  = 0
	socksPortIndex = 0
	portsMutex     = &sync.Mutex{}

	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type ChromeClient struct {
	ctx        context.Context
	httpClient *http.Client
	cb         *gobreaker.CircuitBreaker[any]
	cancel     context.CancelFunc
	proxy      *ProxyConfig
}
type ProxyConfig struct {
	HTTP  string
	HTTPS string
}

func NewChromeClient() *ChromeClient {
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
		chromedp.Flag("log-level", "3"),
	)

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)

	ctx, cancel := chromedp.NewContext(allocCtx)

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 1 * time.Minute,
			DualStack: true,
		}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 5 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       120 * time.Second,
		ForceAttemptHTTP2:     true,
		DisableCompression:    false,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	cb := gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
		Name:        "ChromeClient",
		MaxRequests: 5,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Logger.Info("Circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	})

	return &ChromeClient{
		ctx:        ctx,
		httpClient: httpClient,
		cb:         cb,
		cancel:     cancel,
	}
}

func (cc *ChromeClient) Close() {
	cc.cancel()
}
