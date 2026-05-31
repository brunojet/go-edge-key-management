# go-edge-key-management - Arquitetura (esqueleto)

Objetivo: rotacionar chaves RSA usadas por CloudFront Signed URLs. Comportamento esperado:

- Geração de par RSA (Lambda)
- Persistência do par (Secrets Manager) com `current` e `previous`
- Criação/atualização de `aws_cloudfront_public_key` e `aws_cloudfront_key_group` mantendo current+previous
- Agendamento da rotação via EventBridge

Componentes:

- Lambda rotator (Go)
- AWS Secrets Manager (para private keys)
- CloudFront PublicKey + KeyGroup
- EventBridge rule para agendamento
- Terraform module `key_rotator` para deploy

Operação local: `go run ./cmd/rotator -out ./tmp-output`
