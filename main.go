package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/gomail.v2"

	"github.com/gin-gonic/gin"
)

type EmailRequest struct {
	To         string `json:"to" binding:"required"`
	Subject    string `json:"subject" binding:"required"`
	Body       string `json:"body" binding:"required"`
	Filename   string `json:"filename,omitempty"`
	Attachment string `json:"attachment,omitempty"`
}

type Config struct {
	GmailUser     string
	GmailPassword string
	SmtpHost      string
	SmtpPort      string
}

var appConfig Config

func init() {
	appConfig = Config{
		GmailUser:     os.Getenv("GMAIL_USERNAME"),
		GmailPassword: os.Getenv("GMAIL_APP_PASSWORD"),
		SmtpHost:      "smtp.gmail.com",
		SmtpPort:      "587",
	}

	if appConfig.GmailUser == "" || appConfig.GmailPassword == "" {
		log.Fatal("GMAIL_USERNAME and GMAIL_APP_PASSWORD environment variables must be set")
	}
}

func generateMessageID() string {
	return fmt.Sprintf("%d.%d", time.Now().UnixNano(), rand.Int63())
}

func sendEmailHtmlFormat(to, subject, body, filename, attachment string) error {
	m := gomail.NewMessage()

	sender := appConfig.GmailUser

	m.SetHeader("From", sender)
	m.SetHeader("To", sender, to)
	//m.SetAddressHeader("Cc", "") #caso tenha algum
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	if filename != "" && attachment != "" {
		decodedAttachment, err := base64.StdEncoding.DecodeString(attachment)
		if err != nil {
			return fmt.Errorf("failed to decode attachment: %w", err)
		}

		m.Attach(filename, gomail.SetCopyFunc(func(w io.Writer) error {
			_, err := w.Write(decodedAttachment)
			return err
		}))
	}

	// send email
	port, _ := strconv.Atoi(appConfig.SmtpPort)
	d := gomail.NewDialer(appConfig.SmtpHost, port, appConfig.GmailUser, appConfig.GmailPassword)

	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func sendEmail(to, subject, body, filename, attachment string) error {
	auth := smtp.PlainAuth("", appConfig.GmailUser, appConfig.GmailPassword, appConfig.SmtpHost)

	var emailBuf bytes.Buffer
	writer := multipart.NewWriter(&emailBuf)

	// Cabeçalhos do email
	headers := map[string]string{
		"From":         appConfig.GmailUser,
		"To":           to,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": fmt.Sprintf("multipart/mixed; boundary=%s", writer.Boundary()),
		"Return-Path":  appConfig.GmailUser,
		"Message-ID":   fmt.Sprintf("<%s@%s>", generateMessageID(), appConfig.SmtpHost),
		"X-Mailer":     "TTZ Sistema de Compras",
		"X-Priority":   "1 (Highest)",
	}

	for k, v := range headers {
		emailBuf.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	emailBuf.WriteString("\r\n")

	// Parte de texto
	part, err := writer.CreatePart(map[string][]string{
		"Content-Type": {"text/plain; charset=utf-8"},
	})
	if err != nil {
		return fmt.Errorf("failed to create text part: %v", err)
	}
	part.Write([]byte(strings.ReplaceAll(body, "\\n", "\r\n"))) // Convertendo \n para quebras de linha reais

	// Anexo (se existir)
	if attachment != "" && filename != "" {
		decoded, err := base64.StdEncoding.DecodeString(attachment)
		if err != nil {
			return fmt.Errorf("failed to decode base64 attachment: %v", err)
		}

		contentType := "application/pdf" // Assumindo PDF como padrão
		if !strings.HasSuffix(filename, ".pdf") {
			contentType = "application/octet-stream"
		}

		part, err = writer.CreatePart(map[string][]string{
			"Content-Type":              {contentType},
			"Content-Disposition":       {fmt.Sprintf(`attachment; filename="%s"`, filename)},
			"Content-Transfer-Encoding": {"base64"},
		})
		if err != nil {
			return fmt.Errorf("failed to create attachment part: %v", err)
		}

		encoder := base64.NewEncoder(base64.StdEncoding, part)
		if _, err := encoder.Write(decoded); err != nil {
			return fmt.Errorf("failed to write attachment: %v", err)
		}
		encoder.Close()
	}

	writer.Close()

	// Conexão SMTP
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", appConfig.SmtpHost, appConfig.SmtpPort))
	if err != nil {
		return fmt.Errorf("connection failed: %v", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, appConfig.SmtpHost)
	if err != nil {
		return fmt.Errorf("SMTP client creation failed: %v", err)
	}
	defer client.Close()

	// TLS
	tlsConfig := &tls.Config{
		ServerName: appConfig.SmtpHost,
	}
	if err = client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("TLS handshake failed: %v", err)
	}

	// Autenticação
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("authentication failed: %v", err)
	}

	// Envio
	if err = client.Mail(appConfig.GmailUser); err != nil {
		return fmt.Errorf("sender setup failed: %v", err)
	}
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("recipient setup failed: %v", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data writer failed: %v", err)
	}
	defer w.Close()

	if _, err = emailBuf.WriteTo(w); err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	return nil
}

func emailHandler(c *gin.Context) {
	var request EmailRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := sendEmail(request.To, request.Subject, request.Body, request.Filename, request.Attachment); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email enviado com sucesso"})
}

func emailHtmlHandler(c *gin.Context) {
	var request EmailRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := sendEmailHtmlFormat(request.To, request.Subject, request.Body, request.Filename, request.Attachment); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email enviado com sucesso"})
}

func main() {
	router := gin.Default()

	trustedProxies := []string{
		"192.168.1.0/24",
	}

	err := router.SetTrustedProxies(trustedProxies)
	if err != nil {
		log.Fatalf("error on creating trusted proxies: %v", err)
	}

	router.POST("/send-email", emailHandler)
	router.POST("/send-email-html", emailHtmlHandler)

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(router.Run("0.0.0.0:" + port))
}
