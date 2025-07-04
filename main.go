package main

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"gopkg.in/gomail.v2"

	"github.com/gin-gonic/gin"
	ginprometheus "github.com/zsais/go-gin-prometheus"

	"email-service/metrics"
)

type EmailRequest struct {
	To         string `json:"to" binding:"required"`
	Subject    string `json:"subject" binding:"required"`
	Body       string `json:"body" binding:"required"`
	Filename   string `json:"filename,omitempty"`
	Attachment string `json:"attachment,omitempty"`
}

type SenderConfig struct {
	FromEmail              string
	ServiceAccountEmail    string
	ServiceAccountPassword string
	Provider               string
}

type SMTPConfig struct {
	Host string
	Port string
}

var (
	senderConfigs map[string]SenderConfig
	smtpConfigs   = map[string]SMTPConfig{
		"gmail": {
			Host: "smtp.gmail.com",
			Port: "587",
		},
		"office365": {
			Host: "smtp.office365.com",
			Port: "587",
		},
	}
)

func init() {
	senderConfigs = make(map[string]SenderConfig)

	serviceEmail := os.Getenv("SERVICE_ACCOUNT_EMAIL")
	servicePassword := os.Getenv("SERVICE_ACCOUNT_PASS")
	provider := os.Getenv("SENDER_PROVIDER")

	if serviceEmail == "" || servicePassword == "" {
		log.Fatal("The environment variables SERVICE_ACCOUNT_EMAIL and SERVICE_ACCOUNT_PASS are required.")
	}
	log.Printf("Loding Account Service email configurations: %s", serviceEmail)

	senderNamesEnv := os.Getenv("SENDER_NAMES")
	if senderNamesEnv == "" {
		log.Fatal("The variable SENDER_NAMES is required (ex: 'compras, financeiro').")
	}
	senderNames := strings.Split(senderNamesEnv, ",")

	for _, name := range senderNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		fromEmailEnvKey := fmt.Sprintf("SENDER_%s_EMAIL", strings.ToUpper(name))
		fromEmail := os.Getenv(fromEmailEnvKey)

		if fromEmail == "" {
			log.Fatalf("Email not configured for sender '%s'. Define a var %s", name, fromEmailEnvKey)
		}

		senderConfigs[name] = SenderConfig{
			FromEmail:              fromEmail,
			ServiceAccountEmail:    serviceEmail,
			ServiceAccountPassword: servicePassword,
			Provider:               provider,
		}
		log.Printf("-> Configuring sender '%s' to send as '%s'", name, fromEmail)
	}
}

func sendEmailHtmlFormat(config SenderConfig, to, subject, body, filename, attachment string) error {
	m := gomail.NewMessage()

	m.SetHeader("From", config.FromEmail)
	m.SetHeader("To", to)
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

	smtpConfig, ok := smtpConfigs[config.Provider]
	if !ok {
		return fmt.Errorf("unknown email provider: %s", config.Provider)
	}

	port, _ := strconv.Atoi(smtpConfig.Port)

	d := gomail.NewDialer(smtpConfig.Host, port, config.ServiceAccountEmail, config.ServiceAccountPassword)

	if config.Provider == "office365" {
		d.TLSConfig = &tls.Config{
			ServerName: smtpConfig.Host,
			MinVersion: tls.VersionTLS12,
		}
	}

	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email as '%s' via provider '%s': %w", config.FromEmail, config.Provider, err)
	}

	return nil
}

// Factory patterns for handler
func createEmailHandler(senderName string) gin.HandlerFunc {
	config, ok := senderConfigs[senderName]

	if !ok {
		log.Fatalf("No Configuration found for sender '%s'", senderName)
	}

	return func(c *gin.Context) {
		var request EmailRequest

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err := sendEmailHtmlFormat(config, request.To, request.Subject, request.Body, request.Filename, request.Attachment)

		if err != nil {
			metrics.EmailsProcessed.WithLabelValues(senderName, strconv.Itoa(http.StatusInternalServerError)).Inc()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		metrics.EmailsProcessed.WithLabelValues(senderName, strconv.Itoa(http.StatusOK)).Inc()
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Email From '%s' sent successfully", senderName)})
	}
}

func getIpHandler(c *gin.Context) {
	cmd := exec.Command("hostname", "-i")
	output, err := cmd.Output()

	hostname, _ := os.Hostname()

	var hostnameIP string
	if err == nil {
		hostnameIP = strings.TrimSpace(string(output))
	}

	var dialIP string
	if conn, err := net.Dial("udp", "8.8.8.8"); err == nil {
		defer conn.Close()
		dialIP = conn.LocalAddr().(*net.UDPAddr).IP.String()
	}

	c.JSON(http.StatusOK, gin.H{
		"pod_name":    hostname,
		"server_ip":   hostnameIP,
		"outbound_ip": dialIP,
		"client_ip":   c.ClientIP(),
	})
}

func main() {
	router := gin.Default()

	p := ginprometheus.NewPrometheus("email_service")
	p.Use(router)

	trustedProxies := []string{
		"192.168.1.0/24",
	}

	err := router.SetTrustedProxies(trustedProxies)
	if err != nil {
		log.Fatalf("error on creating trusted proxies: %v", err)
	}

	for name := range senderConfigs {
		endpoint := fmt.Sprintf("/send-email-%s", name)

		router.POST(endpoint, createEmailHandler(name))
		log.Printf("Endpoint registred: POST %s", endpoint)
	}

	router.GET("/get-ip", getIpHandler)

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
