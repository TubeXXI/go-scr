package scraper

import (
	"fmt"
	"math/rand"
	"net/url"
	"sync"
	"time"
)

const (
	XUI_HOST     = "localhost"
	XUI_USERNAME = "infrastructure-admin"
	XUI_PASSWORD = "NX5hJ3nLRAZ8qRjTsx1VUsIDbchBZ0zG"
)

var (
	httpPorts  = []int{3129}
	socksPorts = []int{1080}

	httpPortIndex  = 0
	socksPortIndex = 0
	portsMutex     = &sync.Mutex{}

	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type ProxyConfig struct {
	Server   string // Contoh: "socks5://174.138.75.37:1080"
	Username string
	Password string
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

	var server string
	var port int

	if chosenType == "http" {
		port = getNextHTTPPort()
		server = fmt.Sprintf("http://%s:%s@%s:%d", XUI_USERNAME, XUI_PASSWORD, XUI_HOST, port)
	} else {
		port = getNextSOCKSPort()
		server = fmt.Sprintf("socks5://%s:%s@%s:%d", XUI_USERNAME, XUI_PASSWORD, XUI_HOST, port)
	}

	return &ProxyConfig{
		Server:   server,
		Username: XUI_USERNAME,
		Password: XUI_PASSWORD,
	}
}
func GetHTTPProxyOnly() *ProxyConfig {
	port := getNextHTTPPort()
	server := fmt.Sprintf("http://%s:%d", XUI_HOST, port)

	return &ProxyConfig{
		Server:   server,
		Username: XUI_USERNAME,
		Password: XUI_PASSWORD,
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
