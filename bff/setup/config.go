package setup

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server settings
	BFFListenAddr string
	ProxyTarget   string

	// Authentication (Keycloak/OIDC) settings
	OIDCProviderURL    string
	OAuth2ClientID     string
	OAuth2ClientSecret string
	OAuth2RedirectURL  string
	OAuth2Scopes        []string
	OAuth2TargetAudience string // Audience for token exchange (resource server client ID)

	// Redis settings
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	SessionTTL        time.Duration
	SessionKeyPrefix  string
	StateKeyPrefix    string
	StateTTL          time.Duration // State token TTL for CSRF protection

	// Session/Cookie settings
	SessionCookieName     string
	SessionCookieSecure   bool
	SessionCookieHttpOnly bool
	SessionCookieSameSite string // "Strict", "Lax", "None"

	// CORS settings
	CORSAllowOrigin  string
	CORSAllowMethods string

	// TLS/Certificate settings
	RootCAFile string

	// Security settings
	AllowedRedirectPathPattern string // Regex pattern for allowed redirect URLs
	RedisEncryptionKey         []byte // AES-256-GCM key for Redis session encryption (32 bytes)
}

var (
	config *Config
	once   sync.Once
)

func loadEnvFile() {
	env := os.Getenv("GO_ENV")

	// Production環境では.envファイルを読み込まない（環境変数のみ使用）
	if env == "production" {
		return
	}

	var envFile string
	if env == "test" {
		envFile = ".env.test"
	} else {
		// development or default
		envFile = ".env"
	}

	_ = godotenv.Load(envFile)
}

func GetConfig() *Config {
	once.Do(func() {
		// Load .env file based on GO_ENV (ignore error if file doesn't exist)
		loadEnvFile()

		config = &Config{
			// Server settings
			BFFListenAddr: getEnv("BFF_LISTEN_ADDR", ":8080"),
			ProxyTarget:   getEnv("PROXY_TARGET", "api:8081"),

			// Authentication (Keycloak/OIDC) settings
			OIDCProviderURL:    getEnv("OIDC_PROVIDER_URL", "https://auth.local/idp/realms/go-proxy"),
			OAuth2ClientID:     getEnv("OAUTH2_CLIENT_ID", "bff"),
			OAuth2ClientSecret: getEnv("OAUTH2_CLIENT_SECRET", ""),
			OAuth2RedirectURL:  getEnv("OAUTH2_REDIRECT_URL", "https://auth.local/api/auth/callback"),
			OAuth2Scopes:         getEnvSlice("OAUTH2_SCOPES", []string{"openid", "profile", "email"}),
				OAuth2TargetAudience: getEnv("OAUTH2_TARGET_AUDIENCE", "api"),

			// Redis settings
			RedisAddr:        getEnv("REDIS_ADDR", "redis:6379"),
			RedisPassword:    getEnv("REDIS_PASSWORD", ""),
			RedisDB:          getEnvInt("REDIS_DB", 0),
			SessionTTL:       getEnvDuration("SESSION_TTL", 30*24*time.Hour), // 30 days
			SessionKeyPrefix: getEnv("SESSION_KEY_PREFIX", "session:"),
			StateKeyPrefix:   getEnv("STATE_KEY_PREFIX", "state:"),
			StateTTL:         getEnvDuration("STATE_TTL", 5*time.Minute), // 5 minutes for OAuth2 state

			// Session/Cookie settings
			SessionCookieName:     getEnv("SESSION_COOKIE_NAME", "Session-Id"),
			SessionCookieSecure:   getEnvBool("SESSION_COOKIE_SECURE", true),
			SessionCookieHttpOnly: getEnvBool("SESSION_COOKIE_HTTPONLY", true),
			SessionCookieSameSite: getEnv("SESSION_COOKIE_SAMESITE", "Strict"),

			// CORS settings
			CORSAllowOrigin:  getEnv("CORS_ALLOW_ORIGIN", "https://localhost:3000"),
			CORSAllowMethods: getEnv("CORS_ALLOW_METHODS", "GET, POST, PUT, DELETE, OPTIONS"),

			// TLS/Certificate settings
			RootCAFile: getEnv("ROOT_CA_FILE", "./rootCA.crt"),

			// Security settings
			AllowedRedirectPathPattern: getEnv("ALLOWED_REDIRECT_PATH_PATTERN", ""),
			RedisEncryptionKey:         mustDecodeHexKey("REDIS_ENCRYPTION_KEY"),
		}
	})
	return config
}

// mustDecodeHexKey decodes a 64-char hex string from the given env var into 32 bytes.
// Calls log.Fatal if the env var is missing or not exactly 32 bytes after decoding.
func mustDecodeHexKey(envVar string) []byte {
	raw := os.Getenv(envVar)
	if raw == "" {
		log.Fatalf("required environment variable %s is not set (generate with: openssl rand -hex 32)", envVar)
	}
	key, err := hex.DecodeString(raw)
	if err != nil {
		log.Fatalf("invalid %s: must be a hex-encoded string: %v", envVar, err)
	}
	if len(key) != 32 {
		log.Fatalf("invalid %s: decoded key must be 32 bytes (got %d)", envVar, len(key))
	}
	return key
}

// Validate validates the configuration and compiles regex patterns to detect errors early
func (cfg *Config) Validate() error {
	// Compile redirect path pattern to detect errors at startup
	if cfg.AllowedRedirectPathPattern != "" {
		_, err := regexp.Compile(cfg.AllowedRedirectPathPattern)
		if err != nil {
			return fmt.Errorf("invalid ALLOWED_REDIRECT_PATH_PATTERN: %w", err)
		}
	}
	return nil
}

func getEnv(key string, defaultValue string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	return value
}

func getEnvInt(key string, defaultValue int) int {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return boolValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}
	return duration
}

func getEnvSlice(key string, defaultValue []string) []string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	if value == "" {
		return defaultValue
	}
	return strings.Split(value, ",")
}