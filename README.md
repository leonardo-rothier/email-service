# Email Service
Serviço de envio de emails em Go. Projeto de teste para envio de emails para um sistema de compras, feito em um dia, só para teste. Ainda terá melhorias.

## Como executar
```bash
docker build -t email-service:latest .
docker run --restart=unless-stopped -d -p 8080:8080 -e GMAIL_USERNAME=example.gmail.com -e GMAIL_PASSWORD=P#ssword email-service
```
