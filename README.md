# go-edge-key-management

Esqueleto do projeto `go-edge-key-management` — serviço responsável por gerar e rotacionar pares RSA usados por CloudFront Signed URLs.

Requisitos:

- Go 1.26.3 ou superior (recomendado). Verifique com `go version` ou instale via https://go.dev/dl


Quickstart (local test):

1. Inicialize módulo Go e rode:

```bash
go mod tidy
go run ./cmd/rotator -out ./tmp-output
```

O comando gera um arquivo JSON em `tmp-output/` com a chave atual (local-only, para testes).

Para deploy real, siga os passos em `docs/ARCHITECTURE.md`.
