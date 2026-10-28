package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
	"github.com/marknefedov/go-webpush"
)

func main() {
	writeEnv := flag.Bool("env", false, "Automatically update or append VAPID keys into .env")
	writePEM := flag.Bool("pem", false, "Export private key to vapid_private.pem file")
	flag.Parse()

	_ = godotenv.Load()

	// Resolve contact email from environment
	subject := os.Getenv("VAPID_SUBJECT")
	if subject == "" {
		email := os.Getenv("SMTP_FROM_EMAIL")
		if email == "" {
			email = os.Getenv("SEED_ADMIN_EMAIL")
		}
		if email != "" {
			if !strings.HasPrefix(email, "mailto:") {
				subject = "mailto:" + email
			} else {
				subject = email
			}
		} else {
			subject = "mailto:admin@cadence.local"
		}
	}

	keys, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		log.Fatalf("Failed to generate VAPID keys: %v", err)
	}

	pubKey := keys.PublicKeyString()
	pemBytes, err := keys.ExportVAPIDPrivateKeyPEM()
	if err != nil {
		log.Fatalf("Failed to export PEM private key: %v", err)
	}

	rawJSON, err := keys.MarshalJSON()
	if err != nil {
		log.Fatalf("Failed to marshal keys: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal(rawJSON, &parsed); err != nil {
		log.Fatalf("Failed to parse keys JSON: %v", err)
	}
	privKey := parsed["privateKey"]

	fmt.Println("==================================================")
	fmt.Println("  Cadence Web Push VAPID Key Generator")
	fmt.Println("==================================================")
	fmt.Printf("\nPublic Key:\n%s\n", pubKey)
	fmt.Printf("\nPrivate Key:\n%s\n", privKey)
	fmt.Println("--------------------------------------------------")
	fmt.Println("Values for your .env file:")
	fmt.Printf("VAPID_PUBLIC_KEY=%s\n", pubKey)
	fmt.Printf("VAPID_PRIVATE_KEY=%s\n", privKey)
	fmt.Printf("VAPID_SUBJECT=%s\n", subject)
	fmt.Println("==================================================")

	if *writePEM {
		if err := os.WriteFile("vapid_private.pem", pemBytes, 0600); err != nil {
			log.Fatalf("Failed to write vapid_private.pem: %v", err)
		}
		fmt.Println("Private key written to vapid_private.pem")
	}

	if *writeEnv {
		updateEnvFile(pubKey, privKey, subject)
	}
}

func updateEnvFile(pubKey, privKey, subject string) {
	envPath := ".env"
	content, err := os.ReadFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new .env
			newContent := fmt.Sprintf("VAPID_PUBLIC_KEY=%s\nVAPID_PRIVATE_KEY=%s\nVAPID_SUBJECT=%s\n", pubKey, privKey, subject)
			if err := os.WriteFile(envPath, []byte(newContent), 0644); err != nil {
				log.Fatalf("Failed to create .env: %v", err)
			}
			fmt.Println("Created .env with new VAPID keys!")
			return
		}
		log.Fatalf("Failed to read .env: %v", err)
	}

	envStr := string(content)

	// Replace existing keys if present
	rePub := regexp.MustCompile(`(?m)^VAPID_PUBLIC_KEY=.*$`)
	rePriv := regexp.MustCompile(`(?m)^VAPID_PRIVATE_KEY=.*$`)
	reSub := regexp.MustCompile(`(?m)^VAPID_SUBJECT=.*$`)

	if rePub.MatchString(envStr) {
		envStr = rePub.ReplaceAllString(envStr, "VAPID_PUBLIC_KEY="+pubKey)
	} else {
		envStr += "\nVAPID_PUBLIC_KEY=" + pubKey
	}

	if rePriv.MatchString(envStr) {
		envStr = rePriv.ReplaceAllString(envStr, "VAPID_PRIVATE_KEY="+privKey)
	} else {
		envStr += "\nVAPID_PRIVATE_KEY=" + privKey
	}

	if reSub.MatchString(envStr) {
		envStr = reSub.ReplaceAllString(envStr, "VAPID_SUBJECT="+subject)
	} else {
		envStr += "\nVAPID_SUBJECT=" + subject
	}

	if err := os.WriteFile(envPath, []byte(envStr), 0644); err != nil {
		log.Fatalf("Failed to write .env: %v", err)
	}

	fmt.Println("Updated .env with new VAPID keys!")
}
