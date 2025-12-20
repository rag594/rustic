## Rustic - A minimalistic library in Go for HTTP Client and tracing constructs

Yet another HTTP Client in go with very simple yet essential features

<img width="638" alt="Screenshot 2025-02-09 at 1 10 52 PM" src="https://github.com/user-attachments/assets/cb016287-0016-4de8-800f-84eb890de9ca" />

[![Go Reference](https://pkg.go.dev/badge/github.com/rag594/rustic.svg)](https://pkg.go.dev/github.com/rag594/rustic)

### Package Structure

This project is organized as a **multi-module Go workspace** to prevent unnecessary dependencies and provide clean separation of concerns:

```
rustic/
├── httpClient/           # HTTP client with type safety and circuit breaker support
├── rusticTracer/         # Core OpenTelemetry tracing functionality
├── middleware/
│   ├── echov3/          # Echo v3 tracing middleware (separate module)
│   └── echov4/          # Echo v4 tracing middleware (separate module)
└── example/             # Example applications demonstrating usage
    ├── httpGet/
    ├── httpPost/
    ├── httpPut/
    ├── otlpWithHeaders/
    └── echoTraceMiddleware/
        ├── userService/
        └── postService/
```

**Key Design Principles:**
- **Isolated Dependencies**: Framework-specific middleware (Echo v3/v4) are separate modules, so users only pull Echo dependencies when explicitly needed
- **Modular Architecture**: Each package (`httpClient`, `rusticTracer`, `middleware/echov3`, `middleware/echov4`) is an independent Go module with its own `go.mod`
- **Example Applications**: Complete working examples demonstrate integration patterns

### Features of HTTPClient
- [x] http client with type safety
- [x] Different http configurations support - Timeout, Headers, QueryParams, FormParams, MultipartFormParams, CircuitBreaker
- [x] Supports GET, POST, POSTMultiPartFormData, POSTFormData, PUT
  - [ ] DELETE, PATCH
- [ ] Add metrics either via open telemetry or prometheus metrics
- [ ] Add support for retries, it should have either default/custom or without any retrier

### Features of Tracing constructs
- [x] supports opentelemetry - stdOut and OTLP Http exporter
- [x] tracing middleware for echo v3 and v4 (separate packages to avoid unnecessary deps)

> **_NOTE:_**  For circuit breaker https://github.com/sony/gobreaker is used.

### Installation

Install only the modules you need:

#### HTTP Client Module

```shell
go get github.com/rag594/rustic/httpClient
```

This module provides the HTTP client with type safety, circuit breaker support, and integrated tracing.

#### Tracing Module

```shell
go get github.com/rag594/rustic/rusticTracer
```

Core OpenTelemetry tracing functionality with StdOut and OTLP exporters.

#### Echo Middleware (Optional)

Only install if you're using the Echo framework:

**For Echo v4:**
```shell
go get github.com/rag594/rustic/middleware/echov4
```

**For Echo v3:**
```shell
go get github.com/rag594/rustic/middleware/echov3
```

These middleware packages are separate modules, so Echo dependencies are only pulled when explicitly needed.

### How to use it

#### HTTP Client

```go
// UserPost represents the post/blog by a specific user 
type UserPost struct {
	UserId int    `json:"userId"`
	Id     int    `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}
```

Initialise tracer for HTTP client

```go
shutdown := rusticTracer.InitTracer("microserviceA", "dev", rusticTracer.StdOutExporter())
defer shutdown()
```

```go
// configure your http client
client := httpClient.NewHTTPClient(httpClient.WithTraceEnabled(true))
url := "https://jsonplaceholder.typicode.com/posts"

// define your query params
params := url2.Values{}
params.Add("userId", "1")

// configure your circuit breaker(currently only sony circuit breaker is supported)
st := &gobreaker.Settings{}
st.Name = "HTTP GET"

st.ReadyToTrip = func(counts gobreaker.Counts) bool {
        failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
        return counts.Requests >= 3 && failureRatio >= 0.6
}

cb := gobreaker.NewCircuitBreaker[any](*st)
```

##### Trigger HTTP client

```go
post, err := rustic.GET[[]UserPost](context.Background(),
        url,
        rustic.WithQueryParams(params),
        rustic.WithHttpClient(client),
        rustic.WithTimeout(time.Duration(1)*time.Second),
        rustic.WithCircuitBreaker(cb), 
			)
    if err != nil {
        fmt.Println(err)
    }

    fmt.Println(post)
```

#### Opentelementry Tracing

##### Using with Echo Framework

Initialise the trace with service name, environment and exporter()below is an OTLP exporter with configured telemetry backend. That's it, you have configured the traces
```go
import (
    "github.com/rag594/rustic/middleware/echov4" // or echov3 for Echo v3
    "github.com/rag594/rustic/rusticTracer"
)

// you can try out with tracer.StdOutExporter() in your localhost
shutdown := rusticTracer.InitTracer("userService", "dev", rusticTracer.OTLPExporter("localhost", "4318", nil))

defer shutdown()
e.Use(echov4.TracerMiddleware("userService"))
```

##### OTLP Exporter with Custom Headers

For telemetry backends that require authentication (e.g., Grafana Cloud, Honeycomb), you can pass custom headers:
```go
// Define custom headers for OTLP exporter
headers := map[string]string{
    "Authorization": "Bearer <your-api-token>",
}

// Initialize tracer with OTLP exporter using custom headers
shutdown := rusticTracer.InitTracer("microserviceA", "dev", rusticTracer.OTLPExporter("localhost", "4318", headers))
defer shutdown()

client := httpClient.NewHTTPClient(httpClient.WithTraceEnabled(true))

url := "https://jsonplaceholder.typicode.com/posts"
post, err := rustic.POST[UserPostReq, UserPostResp](context.Background(),
    url,
    userPostReq,
    rustic.WithHttpClient(client),
    rustic.WithTimeout(time.Duration(1)*time.Minute),
)
```

You can run the sample under `example/otlpWithHeaders` to see this in action.

You can run the sample under `example/echoTraceMiddleware` and observe the trace as below:

<img width="638" alt="Screenshot 2025-02-09 at 1 10 52 PM" src="assets/trace-example.png" />


##### Important information wrt context

1. If you pass the context as nil, then context is set as `Background`
2. If you pass the timeout, new context is derived from parent context
3. If you wish to maintain context timeout at the parent level, do not pass timeout
