package app

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

func getenvTrim(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := getenvTrim(key); value != "" {
			return value
		}
	}
	return ""
}

func serverAddr() (string, string) {
	if addr := getenvTrim("JANK_ADDR"); addr != "" {
		return normalizeAddr(addr)
	}

	if port := firstEnv("JANK_PORT", "PORT"); port != "" {
		if !validPort(port) {
			log.Warnf("Invalid port %q; falling back to :9090", port)
			return normalizeAddr(":9090")
		}
		return normalizeAddr(":" + port)
	}

	return normalizeAddr(":9090")
}

func validPort(port string) bool {
	value, err := strconv.Atoi(port)
	if err != nil {
		return false
	}
	return value > 0 && value <= 65535
}

func normalizeAddr(addr string) (string, string) {
	normalized := strings.TrimSpace(addr)
	if normalized == "" {
		normalized = ":9090"
	}

	if _, err := strconv.Atoi(normalized); err == nil {
		normalized = ":" + normalized
	}

	host, port, err := net.SplitHostPort(normalized)
	if err != nil {
		if !strings.Contains(normalized, ":") {
			host = normalized
			port = "9090"
			normalized = net.JoinHostPort(host, port)
		} else {
			log.Warnf("Invalid JANK_ADDR %q; falling back to :9090", addr)
			host = ""
			port = "9090"
			normalized = ":9090"
		}
	}

	displayHost := host
	if displayHost == "" || displayHost == "0.0.0.0" || displayHost == "::" {
		displayHost = "localhost"
	}

	logURL := fmt.Sprintf("http://%s:%s", displayHost, port)
	return normalized, logURL
}
