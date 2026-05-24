# Continuidade: go-edge-key-management

Objetivo
-------
Finalizar e operar o rotator de chaves RSA para CloudFront (projeto: `go-edge-key-management`).

Estado atual (resumo)
---------------------
- Esqueleto do serviço e testes locais: `README.md`
- Código principal: `cmd/rotator/main.go`
- Lógica de geração e helpers: `internal/rotator/rotator.go`
- Integração AWS (SDK v2): `internal/rotator/aws.go`
- Módulo Terraform rotator: `terraform/modules/key_rotator/` (secret + IAM policies)
- Script build: `scripts/build-lambda.sh`
- Arquivo de arquitetura: `docs/ARCHITECTURE.md`

Próximos passos prioritários
----------------------------
1. Validar E2E em sandbox:
   - Build e zip da lambda: `./scripts/build-lambda.sh`
   - Criar secret via Terraform ou manualmente no console com `secret_name`
   - Executar rotação manualmente:

```bash
# Na raiz do repositório
export SECRET_NAME="/go-edge-key-management/rotator"
export NAME_PREFIX="go-edge-key"
go run ./cmd/rotator
```

2. Completar módulo Terraform:
   - Parâmetros: `secret_name`, `rotation_days`, `lambda_zip`
   - Outputs: `secret_arn`, `secret_name`, `lambda_role_arn`

3. Atualizar `media_proxy` para aceitar `signed_urls_key_group_id` externo.

4. Criar pipeline CI:
   - Build Lambda (Linux), zip e armazenar artefato
   - `terraform init` e `terraform apply` para provisionar infraestrutura (configurar backend S3: `brunojet-tfstate`)
   - (Opcional) Aplicar módulos auxiliares antes da stack principal, se existirem

5. Testes:
   - Unitários (helpers do rotator)
   - Integração (sandbox AWS)

Comandos úteis
-------------
```bash
# Na raiz do repositório
go mod tidy
go run ./cmd/rotator -out ./tmp-output            # gerar secret local
./scripts/build-lambda.sh                         # build + zip
# Exemplo AWS (requer credenciais)
export SECRET_NAME="/go-edge-key-management/rotator"
export NAME_PREFIX="go-edge-key"
go run ./cmd/rotator
```

Permissões mínimas (Lambda role)
--------------------------------
- `secretsmanager:GetSecretValue`, `secretsmanager:PutSecretValue`, `secretsmanager:DescribeSecret` (no secret ARN)
- `cloudfront:CreatePublicKey`, `cloudfront:GetPublicKey`, `cloudfront:CreateKeyGroup`, `cloudfront:GetKeyGroup`, `cloudfront:UpdateKeyGroup`
- `logs:CreateLogStream`, `logs:PutLogEvents`

Checklist de migração para o repo final
--------------------------------------
- [X] Copiar `cmd/`, `internal/`, `scripts/`, `terraform/modules/key_rotator/`, `docs/` para o repo final
- [ ] Adicionar CI: build (Go linux), zip, store artifact, `terraform init` + `terraform apply` na infraestrutura
- [ ] Testar rotação manual e validar KeyGroup está sendo referenciado pela distribuição CloudFront

Perguntas em aberto
-------------------
1. Intervalo de rotação desejado? (ex.: `24h`, `7d`)
2. Usaremos CMK gerida pelo cliente ou a chave gerida pelo serviço?
3. Deseja histórico além de `current` + `previous`?
4. Quer endpoint/manual trigger para forçar rotação?

Decisões tomadas
----------------
- **Intervalo de rotação:** `7d` (gerenciado pelo Secrets Manager via `rotation_days`).
- **KMS:** usar chave gerida pelo serviço (sem CMK) para PoC — mais simples e rápido para validar.
- **Histórico de chaves:** manter `current` e `previous` somente.
- **Agendamento:** a rotação é agora gerenciada pelo Secrets Manager utilizando o recurso `aws_secretsmanager_secret_rotation` (parâmetro `rotation_days`).

Observações finais
------------------
Criei este documento como ponto de partida para continuar o trabalho no repo final. Posso também abrir uma Issue template no GitHub com a checklist e passos de rollout se preferir.
