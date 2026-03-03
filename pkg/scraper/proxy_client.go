package scraper

import (
	"fmt"
	"math/rand"
	"net/url"
	"sync"
	"time"
)

const (
	XUI_HOST     = "x-ui.agcforge.com/"
	XUI_USERNAME = "tubexxi"
	XUI_PASSWORD = "ngfbMybEOf"
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
		server = fmt.Sprintf("http://%s:%d", XUI_HOST, port)
	} else {
		port = getNextSOCKSPort()
		server = fmt.Sprintf("socks5://%s:%d", XUI_HOST, port)
	}

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
