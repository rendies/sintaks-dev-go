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

## Embeddings

```go
embeddings, err := client.Embedding(context.Background(), sintaksdev.EmbeddingRequest{
    Input: []string{"Serverless Postgres with sintaks.dev"},
})
if err != nil {
    log.Fatal(err)
}
fmt.Println(len(embeddings.Data[0].Embedding)) // 1024 (qwen3-embedding-0-6b)
```

The default embedding model is `qwen3-embedding-0-6b` (1024 dimensions);
override with `sintaksdev.WithEmbeddingModel("...")`. `encoding_format` and
`dimensions` are supported as optional fields.

Use `ChatCompletionStream` for server-sent events, `GenerateImage` for image
URLs, and `Embedding` for vector embeddings. See the package documentation
and `examples/basic` for more examples.

## Development

```sh
go test ./...
go run ./examples/basic
```

## License

Copyright (c) PT Trakk Digital Indonesia. All rights reserved.
