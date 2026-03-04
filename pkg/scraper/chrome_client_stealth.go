package scraper

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"tubexxi/scraper/pkg/logger"

	"github.com/chromedp/chromedp"
	"github.com/go-rod/stealth"
	"github.com/sony/gobreaker/v2"
	"go.uber.org/zap"
)

const (
	MovieBaseURL  = "https://tv8.lk21official.cc/"
	SeriesBaseURL = "https://tv3.nontondrama.my/"
	AnimeBaseURL  = "https://otakudesu.best/"
)

type ChromeClientStealth struct {
	ctx          context.Context
	httpClient   *http.Client
	cb           *gobreaker.CircuitBreaker[string]
	cancel       context.CancelFunc
	useProxy     bool
	proxyRotator *ProxyRotator
}

func NewChromeClientStealth(useProxy bool) *ChromeClientStealth {
	var proxyRotator *ProxyRotator
	var proxyURL string

	if useProxy {
		proxyRotator = GetProxyRotator()
		proxyRotator.TestAllProxies()

		logger.Logger.Info("Using proxy rotator for Chrome")
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", "new"),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-features", "site-per-process,TranslateUI"),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-notifications", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-default-network-list", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("disable-client-side-phishing-detection", true),
		chromedp.Flag("disable-crash-reporter", true),
		chromedp.Flag("disable-oopr-debug-crash-dump", true),
		chromedp.Flag("no-crash-upload", true),
		chromedp.Flag("disable-low-res-tiling", true),
		chromedp.Flag("log-level", "3"),
		chromedp.Flag("silent-download", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
		chromedp.UserAgent(randomUserAgent()),
		chromedp.Flag("use-gl", "desktop"),
		chromedp.Flag("ignore-certificate-errors", true),
	)

	if proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err == nil {
			opts = append(opts, chromedp.ProxyServer(u.Host))
			logger.Logger.Info("Chrome proxy configured",
				zap.String("proxy_host", u.Host),
			)
		}
	}

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)

	ctx, cancel := context.WithTimeout(allocCtx, 120*time.Second)

	ctx, _ = chromedp.NewContext(ctx)

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
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	cb := gobreaker.NewCircuitBreaker[string](gobreaker.Settings{
		Name:        "ChromeClientStealth",
		MaxRequests: 3,
		Interval:    30 * time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 3
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Logger.Info("Circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	})

	client := &ChromeClientStealth{
		ctx:          ctx,
		httpClient:   httpClient,
		cb:           cb,
		cancel:       cancel,
		useProxy:     useProxy,
		proxyRotator: proxyRotator,
	}

	return client
}

func (cc *ChromeClientStealth) testConnection() error {
	ctx, cancel := context.WithTimeout(cc.ctx, 30*time.Second)
	defer cancel()

	var res string
	err := chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.Evaluate(`"Chrome is working"`, &res),
	)

	if err != nil {
		return fmt.Errorf("chrome connection test failed: %w", err)
	}

	logger.Logger.Info("Chrome connection test successful")
	return nil
}

func SetupProxyAuth(ctx context.Context, proxyConfig *ProxyConfig) context.Context {
	if proxyConfig == nil || proxyConfig.Server == "" {
		return ctx
	}

	ctx, _ = chromedp.NewContext(ctx)

	return ctx
}

func (cc *ChromeClientStealth) Close() {
	if cc.cancel != nil {
		cc.cancel()
	}
}

func (cc *ChromeClientStealth) GetContext() context.Context {
	return cc.ctx
}

func (cc *ChromeClientStealth) NavigateWithRetry(targetURL string, waitTime time.Duration, maxRetries int) (string, error) {
	var htmlContent string
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(cc.ctx, 90*time.Second)

		var tasks chromedp.Tasks
		tasks = append(tasks,
			chromedp.ActionFunc(func(ctx context.Context) error {
				logger.Logger.Debug("Running stealth JS")
				return chromedp.Evaluate(stealth.JS, nil).Do(ctx)
			}),
			chromedp.Navigate(targetURL),
			chromedp.Sleep(waitTime),
			chromedp.OuterHTML("html", &htmlContent),
		)

		logger.Logger.Info("Navigating to URL",
			zap.String("url", targetURL),
			zap.Int("retry", i+1),
			zap.Bool("use_proxy", cc.useProxy),
		)

		err := chromedp.Run(ctx, tasks...)
		cancel()

		if err != nil {
			lastErr = err
			logger.Logger.Warn("Navigation failed",
				zap.Error(err),
				zap.Int("retry", i+1),
			)

			if strings.Contains(err.Error(), "context canceled") {
				logger.Logger.Info("Recreating Chrome context...")
				cc.recreateContext()
			}

			time.Sleep(time.Duration(2+i) * time.Second)
			continue
		}

		if strings.Contains(htmlContent, "Cloudflare") ||
			strings.Contains(htmlContent, "Just a moment") ||
			strings.Contains(htmlContent, "Checking your browser") {
			logger.Logger.Warn("Cloudflare protection detected, retrying",
				zap.Int("retry", i+1),
			)
			time.Sleep(time.Duration(3+i*2) * time.Second)
			continue
		}

		return htmlContent, nil
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func (cc *ChromeClientStealth) recreateContext() {
	cc.cancel()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", "new"),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),
	)

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := context.WithTimeout(allocCtx, 120*time.Second)
	ctx, _ = chromedp.NewContext(ctx)

	cc.ctx = ctx
	cc.cancel = cancel
}

func (cc *ChromeClientStealth) ExecuteScript(script string, result interface{}) error {
	err := chromedp.Run(cc.ctx,
		chromedp.Evaluate(script, result),
	)
	return err
}

func (cc *ChromeClientStealth) WaitForElement(selector string, waitTime time.Duration) error {
	ctx, cancel := context.WithTimeout(cc.ctx, waitTime)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.WaitVisible(selector),
	)
	return err
}

func randomUserAgent() string {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return userAgents[rng.Intn(len(userAgents))]
}

func TestProxyAuth(proxyConfig *ProxyConfig) error {
	if proxyConfig == nil {
		return fmt.Errorf("proxy config is nil")
	}

	proxyURL, err := url.Parse(proxyConfig.Server)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	resp, err := client.Get("http://httpbin.org/ip")
	if err != nil {
		return fmt.Errorf("proxy connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("proxy returned status: %s", resp.Status)
	}

	logger.Logger.Info("Proxy test successful",
		zap.String("proxy", proxyURL.Host),
		zap.Int("status", resp.StatusCode),
	)

	return nil
}
