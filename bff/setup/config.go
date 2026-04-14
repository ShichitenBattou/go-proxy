package setup

import (
	"os"
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
	OAuth2Scopes       []string

	// Redis settings
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	SessionTTL        time.Duration
	SessionKeyPrefix  string
	StateKeyPrefix    string

	// Session/Cookie settings
	// TODO: Add HttpOnly and SameSite attributes for enhanced security
	SessionCookieName   string
	SessionCookieSecure bool

	// CORS settings
	CORSAllowOrigin  string
	CORSAllowMethods string

	// TLS/Certificate settings
	RootCAFile string
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
			OAuth2ClientID:     getEnv("OAUTH2_CLIENT_ID", "api"),
			OAuth2ClientSecret: getEnv("OAUTH2_CLIENT_SECRET", ""),
			OAuth2RedirectURL:  getEnv("OAUTH2_REDIRECT_URL", "https://auth.local/api/auth/callback"),
			OAuth2Scopes:       getEnvSlice("OAUTH2_SCOPES", []string{"openid", "profile", "email"}),

			// Redis settings
			RedisAddr:        getEnv("REDIS_ADDR", "redis:6379"),
			RedisPassword:    getEnv("REDIS_PASSWORD", ""),
			RedisDB:          getEnvInt("REDIS_DB", 0),
			SessionTTL:       getEnvDuration("SESSION_TTL", 30*24*time.Hour), // 30 days
			SessionKeyPrefix: getEnv("SESSION_KEY_PREFIX", "session:"),
			StateKeyPrefix:   getEnv("STATE_KEY_PREFIX", "state:"),

			// Session/Cookie settings
			SessionCookieName:   getEnv("SESSION_COOKIE_NAME", "Session-Id"),
			SessionCookieSecure: getEnvBool("SESSION_COOKIE_SECURE", true),

			// CORS settings
			CORSAllowOrigin:  getEnv("CORS_ALLOW_ORIGIN", "https://localhost:3000"),
			CORSAllowMethods: getEnv("CORS_ALLOW_METHODS", "GET, POST, PUT, DELETE, OPTIONS"),

			// TLS/Certificate settings
			RootCAFile: getEnv("ROOT_CA_FILE", "./rootCA.crt"),
		}
	})
	return config
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