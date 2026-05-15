package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	SessionSecret  string
	DevMode        bool
	DatabaseURL    string

	FabricGatewayEndpoint string
	FabricCertPath        string
	FabricKeyPath         string
	FabricTLSCertPath     string
	FabricChannel         string
	FabricChaincode       string
	FabricMSPID           string

	UploadDir       string
	BusinessBankName string
	BusinessRIB      string
	BusinessAccName  string
}

func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		Port:          getEnv("PORT", "8080"),
		SessionSecret: getEnv("SESSION_SECRET", "dev-secret-change-in-production"),
		DevMode:       getEnv("DEV_MODE", "true") == "true",
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://escrow:escrow@localhost:5432/escrow?sslmode=disable"),

		FabricGatewayEndpoint: getEnv("FABRIC_GATEWAY_ENDPOINT", "localhost:7051"),
		FabricCertPath:        getEnv("FABRIC_CERT_PATH", "./fabric/crypto/appUser-cert.pem"),
		FabricKeyPath:         getEnv("FABRIC_KEY_PATH", "./fabric/crypto/appUser-key.pem"),
		FabricTLSCertPath:     getEnv("FABRIC_TLS_CERT_PATH", "./fabric/crypto/peer-tls.pem"),
		FabricChannel:         getEnv("FABRIC_CHANNEL", "escrow-channel"),
		FabricChaincode:       getEnv("FABRIC_CHAINCODE", "escrow"),
		FabricMSPID:           getEnv("FABRIC_MSP_ID", "PlatformMSP"),

		UploadDir:       getEnv("UPLOAD_DIR", "./uploads"),
		BusinessBankName: getEnv("BUSINESS_BANK_NAME", "Banque Nationale Agricole"),
		BusinessRIB:      getEnv("BUSINESS_RIB", "00000000000000000000"),
		BusinessAccName:  getEnv("BUSINESS_ACCOUNT_NAME", "Escrow.tn SARL"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
