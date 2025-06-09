package interceptors

import "github.com/mark3labs/mcp-go/server"

func Callbacks(logCalls, blockSecrets bool) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		if logCalls {
			next = LogCalls(next)
		}

		if blockSecrets {
			next = BlockSecrets(next)
		}

		return next
	}
}
