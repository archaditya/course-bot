package observability

import (
	"net/http"
    "time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
)

func InitSentry(dsn string, environment string) error {
	return sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      environment,
		Release:          "course-bot@1.0.0",
		TracesSampleRate: 0.1, // 10% of transactions for performance monitoring
	})
}

func CaptureException(err error) {
	sentry.CaptureException(err)
}

func CaptureMessage(message string, level sentry.Level) {
	sentry.CaptureMessage(message, level)
}

func GetSentryHandler() http.Handler {
	return sentryhttp.New(sentryhttp.Options{})
}

func Flush() {
	sentry.Flush(2 * time.Second)
}