package webhooks

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

var privateRanges = []string{
	"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
	"169.254.0.0/16", "0.0.0.0/8", "::1/128", "fe80::/10", "fc00::/7",
}

var parsedPrivateRanges []*net.IPNet

func init() {
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("invalid CIDR in privateRanges: %s", cidr))
		}
		parsedPrivateRanges = append(parsedPrivateRanges, network)
	}
}

func ValidateWebhookURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("invalid URL")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("webhook URL must use http or https scheme")
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("webhook URL must not point to localhost")
	}
	ips, err := net.LookupHost(host)
	if err != nil {
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("webhook URL hostname could not be resolved: %s", host)
		}
		ips = []string{ip.String()}
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		for _, network := range parsedPrivateRanges {
			if network.Contains(ip) {
				return fmt.Errorf("webhook URL must not point to private or internal networks")
			}
		}
	}
	return nil
}
