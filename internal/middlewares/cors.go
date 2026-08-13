package middlewares

import "net/http"

func CORS(allowedOrigins []string) Middleware {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			origin := request.Header.Get("Origin")
			if origin != "" {
				if _, ok := allowed[origin]; !ok {
					if request.Method == http.MethodOptions {
						http.Error(writer, "CORS origin is not allowed", http.StatusForbidden)
						return
					}
				} else {
					writer.Header().Set("Access-Control-Allow-Origin", origin)
					writer.Header().Set("Access-Control-Allow-Credentials", "true")
					writer.Header().Add("Vary", "Origin")
					writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Origin, User-Agent, X-Request-ID")
					writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				}
			}

			if request.Method == http.MethodOptions {
				writer.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(writer, request)
		})
	}
}
