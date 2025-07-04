# Email Service

Serviço simples de envio de e-mails, desenvolvido para automatizar o disparo de relatórios financeiros e de compras para a empresa.  

Ele expõe uma API REST para envio de e-mails com suporte a anexos (base64), múltiplos remetentes configuráveis e métricas via Prometheus.  

---

## Funcionalidades

- Envio de e-mails com corpo em HTML.
- Suporte a anexos em formato **Base64**.
- Configuração de múltiplos remetentes via variáveis de ambiente.
- Métricas Prometheus para monitoramento.
- Endpoints para health check e IP do servidor.

---

## Como rodar

1. **Clone o repositório**
```bash
   git clone https://github.com/sua-empresa/email-service.git
   cd email-service
```
2. **Configure as variaveis de ambiente**
```bash
SERVICE_ACCOUNT_EMAIL=conta@dominio.com
SERVICE_ACCOUNT_PASS=senha_da_conta
SENDER_PROVIDER=office365          
SENDER_NAMES=compras,financeiro

# Para cada sender
SENDER_COMPRAS_EMAIL=compras@dominio.com
SENDER_FINANCEIRO_EMAIL=financeiro@dominio.com
SENDER_FINANCEIRO_EMAIL=cotrole@dominio.com

PORT=8080
```
3. **Execute**
```bash
go run main.go
```

## Exemplo Payload
```json
{
  "to": "destinatario@dominio.com",
  "cc": ["cc1@dominio.com", "cc2@dominio.com"],
  "subject": "Relatório Financeiro",
  "body": "<h1>Segue o relatório</h1>",
  "filename": "relatorio.pdf",
  "attachment": "BASE64_DO_ARQUIVO"
}
```
## Endpoints
| Método | Endpoint                 | Descrição                                     |  
| ------ | ------------------------ | --------------------------------------------- |  
| POST   | `/send-email-compras`    | Envia e-mail usando o remetente "compras".    |  
| POST   | `/send-email-financeiro` | Envia e-mail usando o remetente "financeiro". |  
| POST   | `/send-email-controle`   | Envia e-mail usando o remetente "controle".   |  
| GET    | `/health`                | Verifica se o serviço está saudável.          |  
| GET    | `/get-ip`                | Retorna IPs do servidor e cliente.            |  
