package webhooks

import "testing"

func TestValidateWebhookURL_ValidPublicURLs(t *testing.T) {
	valid := []string{
		"https://example.com/webhook",
		"https://hooks.slack.com/services/T00/B00/xxx",
	}
	for _, u := range valid {
		if err := ValidateWebhookURL(u); err != nil {
			t.Errorf("expected valid URL %q, got error: %v", u, err)
		}
	}
}

func TestValidateWebhookURL_RejectsPrivateIPs(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1/hook",
		"http://localhost/hook",
		"http://10.0.0.1/hook",
		"http://172.16.0.1/hook",
		"http://192.168.1.1/hook",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]/hook",
		"http://0.0.0.0/hook",
	}
	for _, u := range blocked {
		if err := ValidateWebhookURL(u); err == nil {
			t.Errorf("expected blocked URL %q to be rejected", u)
		}
	}
}

func TestValidateWebhookURL_RejectsInvalidSchemes(t *testing.T) {
	blocked := []string{
		"ftp://example.com/file",
		"file:///etc/passwd",
		"gopher://evil.com",
		"javascript:alert(1)",
		"",
		"not-a-url",
	}
	for _, u := range blocked {
		if err := ValidateWebhookURL(u); err == nil {
			t.Errorf("expected blocked URL %q to be rejected", u)
		}
	}
}
