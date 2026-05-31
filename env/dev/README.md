# `env/dev` — Development environment

Conteúdo:
- `.env` — variáveis de ambiente de shell (export). Carregue com `source env/dev/.env`.
- `terraform.tfvars` — arquivo de variáveis para usar com `terraform plan -var-file=env/dev/terraform.tfvars`.

Como usar (exemplo):

```bash
# carregar variáveis de ambiente
source env/dev/.env

# inicializar e aplicar terraform a partir da pasta terraform/
cd terraform
terraform init
terraform plan -var-file=../env/dev/terraform.tfvars
terraform apply -var-file=../env/dev/terraform.tfvars
```

Notas:
- O módulo gera um `secret_name` padrão baseado em `var.name` se `secret_name` não for fornecido: `/<var.name>/rotator`.
- Certifique-se que o bucket S3 `brunojet-tfstate` existe antes de executar `terraform init`.
