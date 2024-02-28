# Telegram Logrus Hook

This hook emits log messages (and corresponding fields) to the Telegram API for [andoma-go/logrus](https://github.com/andoma-go/logrus).

## Installation

Install the package with:

```
go get github.com/andoma-go/logrus-hook-telegram
```

## Usage

See the tests for working examples. Also:

```go
import (
	"time"

	log "github.com/andoma-go/logrus"
	telegramhook "github.com/andoma-go/logrus-hook-telegram"
)

func main() {
	hook, err := telegramhook.NewTelegramHook(
		"MyCoolApp",
		"MYTELEGRAMTOKEN",
		"@mycoolusername",
		telegramhook.WithAsync(true),
		telegramhook.WithTimeout(30 * time.Second),
		telegramhook.WithLevel(logrus.ErrorLevel),
	)
	if err != nil {
		log.Fatalf("Encountered error when creating Telegram hook: %s", err)
	}
	log.AddHook(hook)

	// Receive messages on failures
	log.Errorf("Uh oh...")
	...

}
```

Also you can set custom http.Client to use SOCKS5 proxy for example

```go
import (
	"context"
	"net"
	"net/http"
	"time"

	log "github.com/andoma-go/logrus"
	telegramhook "github.com/andoma-go/logrus-hook-telegram"
	"golang.org/x/net/proxy"
)

func main() {
	dialer, err := proxy.SOCKS5("tcp", "127.0.0.1:54321", nil, proxy.Direct)
	dialContext := func(ctx context.Context, network, address string) (net.Conn, error) {
		return dialer.Dial(network, address)
	}
	httpTransport := &http.Transport{
		DialContext:       dialContext,
		DisableKeepAlives: true,
	}
	httpClient := &http.Client{Transport: httpTransport}

	hook, err := telegramhook.NewTelegramHookWithClient(
		"MyCoolApp",
		"MYTELEGRAMTOKEN",
		"@mycoolusername",
		httpClient,
		telegramhook.WithAsync(true),
		telegramhook.WithTimeout(30 * time.Second),
	)
	if err != nil {
		log.Fatalf("Encountered error when creating Telegram hook: %s", err)
	}
	log.AddHook(hook)

	// Receive messages on failures
	log.Errorf("Uh oh...")
	...

}
```
