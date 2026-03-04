package middleware

import "github.com/cloudwego/hertz/pkg/app"

func TraceIDFromRequest(c *app.RequestContext) string {
	if traceID := string(c.GetHeader("X-Trace-Id")); traceID != "" {
		return traceID
	}
	return string(c.GetHeader("x-trace-id"))
}
