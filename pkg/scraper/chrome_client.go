package scraper

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
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
	XUI_HOST      = "x-ui"
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

type ProxyConfig struct {
	HTTP  string
	HTTPS string
}

func (p *ProxyConfig) URL() *url.URL {
	if p == nil {
		return nil
	}
	proxyURL := p.HTTPS
	if proxyURL == "" {
		proxyURL = p.HTTP
	}
	parsed, _ := url.Parse(proxyURL)
	return parsed
}

func (p *ProxyConfig) String() string {
	if p == nil {
		return "no proxy"
	}
	return fmt.Sprintf("HTTP: %s, HTTPS: %s", maskProxyPassword(p.HTTP), maskProxyPassword(p.HTTPS))
}

func maskProxyPassword(proxyStr string) string {
	if proxyStr == "" {
		return ""
	}
	u, err := url.Parse(proxyStr)
	if err != nil {
		return proxyStr
	}
	if u.User != nil {
		return fmt.Sprintf("%s://%s:***@%s", u.Scheme, u.User.Username(), u.Host)
	}
	return proxyStr
}

type ChromeClient struct {
	ctx        context.Context
	httpClient *http.Client
	cb         *gobreaker.CircuitBreaker[any]
	cancel     context.CancelFunc
	proxy      *ProxyConfig
	proxyMutex sync.RWMutex
}

func GetProxyRotating() *ProxyConfig {
	if !USE_XUI_PROXY {
		return nil
	}

	proxyTypes := []string{"http", "socks"}
	chosenType := proxyTypes[rng.Intn(len(proxyTypes))]

	if chosenType == "http" {
		port := getNextHTTPPort()
		return &ProxyConfig{
			HTTP:  fmt.Sprintf("http://%s:%s@%s:%d", XUI_USERNAME, XUI_PASSWORD, XUI_HOST, port),
			HTTPS: fmt.Sprintf("http://%s:%s@%s:%d", XUI_USERNAME, XUI_PASSWORD, XUI_HOST, port),
		}
	} else {
		port := getNextSOCKSPort()
		return &ProxyConfig{
			HTTP:  fmt.Sprintf("socks5://%s:%s@%s:%d", XUI_USERNAME, XUI_PASSWORD, XUI_HOST, port),
			HTTPS: fmt.Sprintf("socks5://%s:%s@%s:%d", XUI_USERNAME, XUI_PASSWORD, XUI_HOST, port),
		}
	}
}

func getNextHTTPPort() int {
	portsMutex.Lock()
	defer portsMutex.Unlock()

	port := httpPorts[httpPortIndex]
	httpPortIndex = (httpPortIndex + 1) % len(httpPorts)
	return port
}

func getNextSOCKSPort() int {
	portsMutex.Lock()
	defer portsMutex.Unlock()

	port := socksPorts[socksPortIndex]
	socksPortIndex = (socksPortIndex + 1) % len(socksPorts)
	return port
}

func (cc *ChromeClient) UpdateProxy() {
	cc.proxyMutex.Lock()
	defer cc.proxyMutex.Unlock()

	newProxy := GetProxyRotating()
	if newProxy != nil {
		cc.proxy = newProxy

		if transport, ok := cc.httpClient.Transport.(*http.Transport); ok {
			transport.Proxy = http.ProxyURL(newProxy.URL())
		}

		logger.Logger.Debug("Proxy updated",
			zap.String("proxy_http", maskProxyPassword(newProxy.HTTP)),
			zap.String("proxy_https", maskProxyPassword(newProxy.HTTPS)),
		)
	}
}

func (cc *ChromeClient) GetCurrentProxy() *ProxyConfig {
	cc.proxyMutex.RLock()
	defer cc.proxyMutex.RUnlock()
	return cc.proxy
}

func (cc *ChromeClient) createChromeOpts() []chromedp.ExecAllocatorOption {
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

	cc.proxyMutex.RLock()
	if cc.proxy != nil && USE_XUI_PROXY {
		if strings.Contains(cc.proxy.HTTP, "http://") {
			proxyURL := cc.proxy.URL()
			if proxyURL != nil {
				proxyHost := fmt.Sprintf("%s:%s", XUI_HOST, cc.getPortFromProxy(cc.proxy.HTTP))
				opts = append(opts, chromedp.ProxyServer(proxyHost))

				logger.Logger.Debug("Using HTTP proxy for Chrome",
					zap.String("proxy_host", proxyHost),
				)
			}
		} else {
			logger.Logger.Debug("Skipping SOCKS proxy for Chrome (not supported)",
				zap.String("proxy_type", getProxyType(cc.proxy)),
			)
		}
	}
	cc.proxyMutex.RUnlock()

	return opts
}

func NewChromeClient() *ChromeClient {
	initialProxy := GetProxyRotating()

	transport := &http.Transport{
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

	if initialProxy != nil {
		transport.Proxy = http.ProxyURL(initialProxy.URL())
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	client := &ChromeClient{
		httpClient: httpClient,
		proxy:      initialProxy,
		proxyMutex: sync.RWMutex{},
	}

	opts := client.createChromeOpts()
	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)

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

	client.ctx = ctx
	client.cancel = cancel
	client.cb = cb

	if initialProxy != nil {
		logger.Logger.Info("ChromeClient initialized with proxy",
			zap.String("proxy_type", getProxyType(initialProxy)),
			zap.String("proxy", maskProxyPassword(initialProxy.HTTP)),
		)
	} else {
		logger.Logger.Info("ChromeClient initialized without proxy")
	}

	return client
}

func (cc *ChromeClient) RotateProxy() *ProxyConfig {
	cc.UpdateProxy()
	cc.recreateChromeContext()

	return cc.GetCurrentProxy()
}

func (cc *ChromeClient) recreateChromeContext() {
	if cc.cancel != nil {
		cc.cancel()
	}

	opts := cc.createChromeOpts()
	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)

	cc.ctx = ctx
	cc.cancel = cancel

	logger.Logger.Debug("Chrome context recreated with new proxy")
}

func getProxyType(p *ProxyConfig) string {
	if p == nil {
		return "none"
	}
	if strings.Contains(p.HTTP, "socks5") {
		return "socks5"
	}
	return "http"
}

func (cc *ChromeClient) WithProxy(fn func() error) error {

	cc.RotateProxy()

	err := fn()

	if err != nil {
		logger.Logger.Error("Function execution failed with proxy",
			zap.Error(err),
			zap.String("proxy", maskProxyPassword(cc.proxy.HTTP)),
		)
	} else {
		logger.Logger.Debug("Function executed successfully with proxy",
			zap.String("proxy", maskProxyPassword(cc.proxy.HTTP)),
		)
	}

	return err
}
func (cc *ChromeClient) getPortFromProxy(proxyStr string) string {
	u, err := url.Parse(proxyStr)
	if err != nil {
		return "1080" // default
	}
	return u.Port()
}
func (cc *ChromeClient) Close() {
	if cc.cancel != nil {
		cc.cancel()
	}
	if cc.httpClient != nil {
		cc.httpClient.CloseIdleConnections()
	}
}
