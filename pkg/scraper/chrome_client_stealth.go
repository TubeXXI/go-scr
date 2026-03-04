package scraper

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"tubexxi/scraper/pkg/logger"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
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
	ctx         context.Context
	httpClient  *http.Client
	cb          *gobreaker.CircuitBreaker[string]
	cancel      context.CancelFunc
	useProxy    bool
	proxyConfig *ProxyConfig
}

func NewChromeClientStealth(useProxy bool) *ChromeClientStealth {
	var proxyConfig *ProxyConfig
	var proxyServer string

	if useProxy {
		proxyConfig = GetHTTPProxyOnly()
		if proxyConfig != nil {
			proxyServer = proxyConfig.Server
			logger.Logger.Info("Using proxy for Chrome",
				zap.String("proxy", proxyServer),
				zap.String("username", proxyConfig.Username),
			)
		}
	}

	if proxyConfig != nil {
		if err := TestProxyAuth(proxyConfig); err != nil {
			logger.Logger.Warn("Proxy test failed, but continuing...",
				zap.Error(err),
			)
		}
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
		// More stealth flags
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
		// Random window size
		chromedp.WindowSize(1920, 1080),
		// Custom user agent
		chromedp.UserAgent(randomUserAgent()),
		// Disable automation detection
		chromedp.Flag("use-gl", "desktop"),
		chromedp.Flag("ignore-certificate-errors", true),
	)

	if proxyServer != "" {
		u, err := url.Parse(proxyServer)
		if err == nil {
			opts = append(opts, chromedp.ProxyServer(u.Host))
			logger.Logger.Info("Chrome proxy configured",
				zap.String("proxy_host", u.Host),
			)
		}
	}

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)

	if proxyConfig != nil {
		var err error
		ctx, err = SetupProxyWithAuth(ctx, proxyConfig)
		if err != nil {
			logger.Logger.Error("Failed to setup proxy auth",
				zap.Error(err),
			)
		}
	}

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

	return &ChromeClientStealth{
		ctx:         ctx,
		httpClient:  httpClient,
		cb:          cb,
		cancel:      cancel,
		proxyConfig: proxyConfig,
	}
}

func NewChromeClientStealthWithProxy(proxyURL string) *ChromeClientStealth {
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
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("disable-client-side-phishing-detection", true),
		chromedp.Flag("disable-crash-reporter", true),
		chromedp.Flag("disable-low-res-tiling", true),
		chromedp.Flag("log-level", "3"),
		chromedp.Flag("silent-download", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("use-gl", "desktop"),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.WindowSize(1920, 1080),
		chromedp.UserAgent(randomUserAgent()),
	)

	if proxyURL != "" {
		opts = append(opts, chromedp.ProxyServer(proxyURL))
	}

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)

	ctx, cancel := chromedp.NewContext(allocCtx)

	var transport *http.Transport
	if proxyURL != "" {
		transport = &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse(proxyURL)
			},
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 1 * time.Minute,
			}).DialContext,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		}
	} else {
		transport = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 1 * time.Minute,
			}).DialContext,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		}
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
	})

	return &ChromeClientStealth{
		ctx:        ctx,
		httpClient: httpClient,
		cb:         cb,
		cancel:     cancel,
	}
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
		ctx, cancel := context.WithTimeout(cc.ctx, 60*time.Second)

		var tasks chromedp.Tasks

		tasks = append(tasks,
			chromedp.Evaluate(stealth.JS, nil),
			chromedp.Navigate(targetURL),
			chromedp.Sleep(waitTime),
			chromedp.OuterHTML("html", &htmlContent),
		)

		logger.Logger.Info("Navigating to URL", zap.String("url", targetURL), zap.Int("retry", i+1))

		err := chromedp.Run(ctx, tasks...)
		cancel()

		if err != nil {
			lastErr = err
			logger.Logger.Warn("Navigation failed", zap.Error(err), zap.Int("retry", i+1))

			if strings.Contains(err.Error(), "ERR_INVALID_AUTH_CREDENTIALS") {
				logger.Logger.Error("Invalid proxy credentials",
					zap.String("username", cc.proxyConfig.Username),
				)
				if cc.useProxy {
					cc.proxyConfig = GetHTTPProxyOnly()
					logger.Logger.Info("Proxy rotated",
						zap.String("new_proxy", cc.proxyConfig.Server),
					)
				}
			}

			time.Sleep(time.Duration(2+i) * time.Second)
			continue
		}

		if strings.Contains(htmlContent, "Cloudflare") ||
			strings.Contains(htmlContent, "Just a moment") ||
			strings.Contains(htmlContent, "Checking your browser") {
			logger.Logger.Warn("Cloudflare protection detected, retrying", zap.Int("retry", i+1))
			time.Sleep(time.Duration(3+i*2) * time.Second)
			continue
		}

		return htmlContent, nil
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
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

func (cc *ChromeClientStealth) SetExtraHeaders(headers map[string]string) error {
	headerMap := make(network.Headers)
	for k, v := range headers {
		headerMap[k] = v
	}
	ctx, cancel := context.WithTimeout(cc.ctx, 10*time.Second)
	defer cancel()
	return chromedp.Run(ctx, network.SetExtraHTTPHeaders(headerMap))
}

func randomUserAgent() string {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return userAgents[rng.Intn(len(userAgents))]
}

func SetupProxyWithAuth(ctx context.Context, proxyConfig *ProxyConfig) (context.Context, error) {
	if proxyConfig == nil || proxyConfig.Server == "" {
		return ctx, nil
	}

	proxyURL, err := url.Parse(proxyConfig.Server)
	if err != nil {
		return ctx, fmt.Errorf("invalid proxy URL: %w", err)
	}

	if err := chromedp.Run(ctx,
		fetch.Enable(),
	); err != nil {
		logger.Logger.Warn("Failed to enable fetch", zap.Error(err))
	}

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *fetch.EventAuthRequired:
			handleAuthRequired(ctx, e, proxyConfig)
		case *fetch.EventRequestPaused:
			handleRequestPaused(ctx, e)
		}
	})

	headers := network.Headers{
		"Proxy-Authorization": basicAuth(proxyConfig.Username, proxyConfig.Password),
	}

	if err := chromedp.Run(ctx,
		network.SetExtraHTTPHeaders(headers),
	); err != nil {
		logger.Logger.Warn("Failed to set extra headers", zap.Error(err))
	}

	logger.Logger.Info("Proxy auth setup completed",
		zap.String("proxy", proxyURL.Host),
		zap.String("username", proxyConfig.Username),
	)

	return ctx, nil
}

func handleAuthRequired(ctx context.Context, event *fetch.EventAuthRequired, proxyConfig *ProxyConfig) {
	logger.Logger.Info("Proxy authentication required",
		zap.String("url", event.Request.URL),
	)

	response := &fetch.AuthChallengeResponse{
		Response: "ProvideCredentials",
		Username: proxyConfig.Username,
		Password: proxyConfig.Password,
	}

	go func() {
		authCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := chromedp.Run(authCtx,
			fetch.ContinueWithAuth(event.RequestID, response),
		); err != nil {
			logger.Logger.Error("Failed to provide proxy auth",
				zap.Error(err),
				zap.String("username", proxyConfig.Username),
			)
		} else {
			logger.Logger.Info("Proxy authentication provided successfully")
		}
	}()
}

func handleRequestPaused(ctx context.Context, event *fetch.EventRequestPaused) {
	go func() {
		pauseCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := chromedp.Run(pauseCtx,
			fetch.ContinueRequest(event.RequestID),
		); err != nil {
			logger.Logger.Debug("Failed to continue request",
				zap.Error(err),
				zap.String("url", event.Request.URL),
			)
		}
	}()
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
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
