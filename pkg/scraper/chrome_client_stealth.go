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
	if useProxy {
		proxyConfig = GetProxyRotating()
	}

	var proxyURL string
	if proxyConfig != nil {
		proxyURL = proxyConfig.Server
		logger.Logger.Info("Using rotating proxy", zap.String("proxy", maskProxyPassword(proxyURL)))
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

	if proxyURL != "" {
		opts = append(opts, chromedp.ProxyServer(proxyURL))
	}

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)

	if useProxy && proxyConfig != nil {
		chromedp.ListenTarget(ctx, func(ev interface{}) {
			if auth, ok := ev.(*fetch.EventAuthRequired); ok {
				go func() {
					err := chromedp.Run(ctx, fetch.ContinueWithAuth(auth.RequestID, &fetch.AuthChallengeResponse{
						Response: "ProvideCredentials",
						Username: proxyConfig.Username,
						Password: proxyConfig.Password,
					}))
					if err != nil {
						logger.Logger.Error("Proxy Auth Failed", zap.Error(err))
					}
				}()
			}
		})
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

// NavigateWithRetry navigates to a URL with retry logic for Cloudflare
func (cc *ChromeClientStealth) NavigateWithRetry(targetURL string, waitTime time.Duration, maxRetries int) (string, error) {
	var htmlContent string
	var err error

	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(cc.ctx, 60*time.Second)

		var tasks chromedp.Tasks
		if cc.useProxy {
			tasks = append(tasks, fetch.Enable())
		}

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
			logger.Logger.Warn("Navigation failed", zap.Error(err), zap.Int("retry", i+1))
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

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, err)
}

// ExecuteScript executes JavaScript and returns the result
func (cc *ChromeClientStealth) ExecuteScript(script string, result interface{}) error {
	err := chromedp.Run(cc.ctx,
		chromedp.Evaluate(script, result),
	)
	return err
}

// WaitForElement waits for a specific element to appear
func (cc *ChromeClientStealth) WaitForElement(selector string, waitTime time.Duration) error {
	ctx, cancel := context.WithTimeout(cc.ctx, waitTime)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.WaitVisible(selector),
	)
	return err
}

// SetExtraHeaders sets extra HTTP headers for the request
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
