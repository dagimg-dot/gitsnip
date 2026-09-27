package downloader

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strconv"
)

func authEnv(remote, token string) []string {
	if token == "" {
		return nil
	}
	u, err := url.Parse(remote)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil
	}

	user := "oauth2"
	if u.Hostname() == "github.com" {
		user = "x-access-token"
	}

	n, err := strconv.Atoi(os.Getenv("GIT_CONFIG_COUNT"))
	if err != nil || n < 0 {
		n = 0
	}

	credentials := base64.StdEncoding.EncodeToString([]byte(user + ":" + token))
	return []string{
		fmt.Sprintf("GIT_CONFIG_COUNT=%d", n+1),
		fmt.Sprintf("GIT_CONFIG_KEY_%d=http.https://%s/.extraheader", n, u.Host),
		fmt.Sprintf("GIT_CONFIG_VALUE_%d=Authorization: Basic %s", n, credentials),
	}
}
