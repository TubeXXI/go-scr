package scraper

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"
	"tubexxi/scraper/pkg/logger"

	"go.uber.org/zap"
)

const (
	XUI_HOST     = "localhost" // change infrastructure-3proxy for prod
	XUI_USERNAME = "infrastructure-admin"
	XUI_PASSWORD = "NX5hJ3nLRAZ8qRjTsx1VUsIDbchBZ0zG"
)

var (
	httpPorts       = []int{3129}
	socksPorts      = []int{1080}
	httpPortIndex   = 0
	socksPortIndex  = 0
	portsMutex      = &sync.Mutex{}
	rng             = rand.New(rand.NewSource(time.Now().UnixNano()))
	rotatorInstance *ProxyRotator
	rotatorOnce     sync.Once
)

type ProxyConfig struct {
	Server   string // Dengan auth: http://user:pass@host:port
	Username string
	Password string
	Host     string
	Port     int
	Scheme   string
}

type ProxyRotator struct {
	proxies     []*ProxyConfig
	current     int
	mu          sync.Mutex
	httpClients map[string]*http.Client
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

func GetProxyRotating() *ProxyConfig {
	proxyTypes := []string{"http", "socks"}
	chosenType := proxyTypes[rng.Intn(len(proxyTypes))]

	if chosenType == "http" {
		return GetHTTPProxy()
	}
	return GetSOCKSProxy()
}

func GetHTTPProxy() *ProxyConfig {
	port := getNextHTTPPort()
	server := fmt.Sprintf("http://%s:%s@%s:%d", XUI_USERNAME, XUI_PASSWORD, XUI_HOST, port)

	return &ProxyConfig{
		Server:   server,
		Username: XUI_USERNAME,
		Password: XUI_PASSWORD,
		Host:     XUI_HOST,
		Port:     port,
		Scheme:   "http",
	}
}

func GetSOCKSProxy() *ProxyConfig {
	port := getNextSOCKSPort()
	server := fmt.Sprintf("socks5://%s:%s@%s:%d", XUI_USERNAME, XUI_PASSWORD, XUI_HOST, port)

	return &ProxyConfig{
		Server:   server,
		Username: XUI_USERNAME,
		Password: XUI_PASSWORD,
		Host:     XUI_HOST,
		Port:     port,
		Scheme:   "socks5",
	}
}

func GetHTTPProxyOnly() *ProxyConfig {
	return GetHTTPProxy()
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

// Proxy Rotator

func GetProxyRotator() *ProxyRotator {
	rotatorOnce.Do(func() {
		proxies := []*ProxyConfig{
			{
				Server:   fmt.Sprintf("http://%s:%s@%s:%d", XUI_USERNAME, XUI_PASSWORD, XUI_HOST, 3129),
				Username: XUI_USERNAME,
				Password: XUI_PASSWORD,
				Host:     XUI_HOST,
				Port:     3129,
				Scheme:   "http",
			},
		}

		rotatorInstance = &ProxyRotator{
			proxies:     proxies,
			current:     0,
			httpClients: make(map[string]*http.Client),
		}
	})
	return rotatorInstance
}
func (pr *ProxyRotator) GetNextProxy() *ProxyConfig {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	proxy := pr.proxies[pr.current]
	pr.current = (pr.current + 1) % len(pr.proxies)
	return proxy
}
func (pr *ProxyRotator) GetHTTPClient() *http.Client {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	proxy := pr.proxies[pr.current]
	key := fmt.Sprintf("%s:%d", proxy.Host, proxy.Port)

	if client, exists := pr.httpClients[key]; exists {
		return client
	}

	proxyURL, _ := url.Parse(proxy.Server)
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	pr.httpClients[key] = client
	return client
}
func (pr *ProxyRotator) TestAllProxies() {
	for _, proxy := range pr.proxies {
		client := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(mustParse(proxy.Server)),
			},
			Timeout: 10 * time.Second,
		}

		resp, err := client.Get("http://httpbin.org/ip")
		if err != nil {
			logger.Logger.Warn("Proxy test failed",
				zap.String("proxy", proxy.Server),
				zap.Error(err),
			)
		} else {
			resp.Body.Close()
			logger.Logger.Info("Proxy test successful",
				zap.String("proxy", proxy.Server),
				zap.Int("status", resp.StatusCode),
			)
		}
	}
}

func mustParse(s string) *url.URL {
	u, _ := url.Parse(s)
	return u
}
