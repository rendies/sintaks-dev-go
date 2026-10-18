# sintaks.dev Go SDK

Official Go client for the [sintaks.dev](https://sintaks.dev) OpenAI-compatible
API gateway. The SDK uses only the Go standard library.

## Install

```sh
go get github.com/rendies/sintaks-dev-go
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    sintaksdev "github.com/rendies/sintaks-dev-go"
)

func main() {
    client := sintaksdev.New("sk-...")
    response, err := client.ChatCompletion(context.Background(), sintaksdev.ChatRequest{
        Model: "claude-sonnet-5-5",
        Messages: []sintaksdev.Message{{Role: "user", Content: "Halo!"}},
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(response.Content())
}
```

Use `ChatCompletionStream` for server-sent events and `GenerateImage` for image
URLs. See the package documentation and `examples/basic` for more examples.

## Development

```sh
go test ./...
go run ./examples/basic
```

## License

Copyright (c) PT Trakk Digital Indonesia. All rights reserved.
