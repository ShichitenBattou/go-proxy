package setup

import (
	"os"
	"sync"
	"testing"
	"time"
)

func resetSingleton() {
	config = nil
	once = sync.Once{}
}

func TestGetConfig(t *testing.T) {
	resetSingleton()
	// Clear all environment variables
	os.Unsetenv("BFF_LISTEN_ADDR")
	os.Unsetenv("PROXY_TARGET")
	os.Unsetenv("OIDC_PROVIDER_URL")
	os.Unsetenv("OAUTH2_CLIENT_ID")
	os.Unsetenv("OAUTH2_CLIENT_SECRET")
	os.Unsetenv("OAUTH2_REDIRECT_URL")
	os.Unsetenv("OAUTH2_SCOPES")
	os.Unsetenv("REDIS_ADDR")
	os.Unsetenv("REDIS_PASSWORD")
	os.Unsetenv("REDIS_DB")
	os.Unsetenv("SESSION_TTL")
	os.Unsetenv("SESSION_KEY_PREFIX")
	os.Unsetenv("STATE_KEY_PREFIX")
	os.Unsetenv("SESSION_COOKIE_NAME")
	os.Unsetenv("SESSION_COOKIE_SECURE")
	os.Unsetenv("CORS_ALLOW_ORIGIN")
	os.Unsetenv("CORS_ALLOW_METHODS")
	os.Unsetenv("ROOT_CA_FILE")

	cfg := GetConfig()

	// Server settings
	if cfg.BFFListenAddr != ":8080" {
		t.Errorf("Expected default BFFListenAddr to be ':8080', got '%s'", cfg.BFFListenAddr)
	}
	if cfg.ProxyTarget != "api:8081" {
		t.Errorf("Expected default ProxyTarget to be 'api:8081', got '%s'", cfg.ProxyTarget)
	}

	// Authentication settings
	if cfg.OIDCProviderURL != "https://auth.local/idp/realms/go-proxy" {
		t.Errorf("Expected default OIDCProviderURL, got '%s'", cfg.OIDCProviderURL)
	}
	if cfg.OAuth2ClientID != "bff" {
		t.Errorf("Expected default OAuth2ClientID to be 'bff', got '%s'", cfg.OAuth2ClientID)
	}

	// Redis settings
	if cfg.RedisAddr != "redis:6379" {
		t.Errorf("Expected default RedisAddr to be 'redis:6379', got '%s'", cfg.RedisAddr)
	}
	if cfg.RedisPassword != "" {
		t.Errorf("Expected default RedisPassword to be empty, got '%s'", cfg.RedisPassword)
	}
	if cfg.RedisDB != 0 {
		t.Errorf("Expected default RedisDB to be 0, got %d", cfg.RedisDB)
	}
	if cfg.SessionTTL != 30*24*time.Hour {
		t.Errorf("Expected default SessionTTL to be 30 days, got %v", cfg.SessionTTL)
	}
	if cfg.SessionKeyPrefix != "session:" {
		t.Errorf("Expected default SessionKeyPrefix to be 'session:', got '%s'", cfg.SessionKeyPrefix)
	}
	if cfg.StateKeyPrefix != "state:" {
		t.Errorf("Expected default StateKeyPrefix to be 'state:', got '%s'", cfg.StateKeyPrefix)
	}

	// Session/Cookie settings
	if cfg.SessionCookieName != "Session-Id" {
		t.Errorf("Expected default SessionCookieName to be 'Session-Id', got '%s'", cfg.SessionCookieName)
	}
	if cfg.SessionCookieSecure != true {
		t.Errorf("Expected default SessionCookieSecure to be true, got %v", cfg.SessionCookieSecure)
	}

	// CORS settings
	if cfg.CORSAllowOrigin != "https://localhost:3000" {
		t.Errorf("Expected default CORSAllowOrigin, got '%s'", cfg.CORSAllowOrigin)
	}

	// TLS settings
	if cfg.RootCAFile != "./rootCA.crt" {
		t.Errorf("Expected default RootCAFile to be './rootCA.crt', got '%s'", cfg.RootCAFile)
	}
}

func TestGetConfigWithEnv(t *testing.T) {
	resetSingleton()
	os.Setenv("BFF_LISTEN_ADDR", ":9090")
	os.Setenv("PROXY_TARGET", "http://test.com")
	os.Setenv("OIDC_PROVIDER_URL", "https://test.local/realms/test")
	os.Setenv("OAUTH2_CLIENT_ID", "test-client")
	os.Setenv("OAUTH2_CLIENT_SECRET", "test-secret")
	os.Setenv("OAUTH2_REDIRECT_URL", "https://test.local/callback")
	os.Setenv("OAUTH2_SCOPES", "openid,profile")
	os.Setenv("REDIS_ADDR", "localhost:6379")
	os.Setenv("REDIS_PASSWORD", "test-password")
	os.Setenv("REDIS_DB", "1")
	os.Setenv("SESSION_TTL", "24h")
	os.Setenv("SESSION_KEY_PREFIX", "test-session:")
	os.Setenv("STATE_KEY_PREFIX", "test-state:")
	os.Setenv("SESSION_COOKIE_NAME", "TestSession")
	os.Setenv("SESSION_COOKIE_SECURE", "false")
	os.Setenv("CORS_ALLOW_ORIGIN", "https://test.local")
	os.Setenv("CORS_ALLOW_METHODS", "GET, POST")
	os.Setenv("ROOT_CA_FILE", "/path/to/ca.crt")

	cfg := GetConfig()

	// Server settings
	if cfg.BFFListenAddr != ":9090" {
		t.Errorf("Expected BFFListenAddr to be ':9090', got '%s'", cfg.BFFListenAddr)
	}
	if cfg.ProxyTarget != "http://test.com" {
		t.Errorf("Expected ProxyTarget to be 'http://test.com', got '%s'", cfg.ProxyTarget)
	}

	// Authentication settings
	if cfg.OIDCProviderURL != "https://test.local/realms/test" {
		t.Errorf("Expected OIDCProviderURL to be 'https://test.local/realms/test', got '%s'", cfg.OIDCProviderURL)
	}
	if cfg.OAuth2ClientID != "test-client" {
		t.Errorf("Expected OAuth2ClientID to be 'test-client', got '%s'", cfg.OAuth2ClientID)
	}
	if cfg.OAuth2ClientSecret != "test-secret" {
		t.Errorf("Expected OAuth2ClientSecret to be 'test-secret', got '%s'", cfg.OAuth2ClientSecret)
	}
	if cfg.OAuth2RedirectURL != "https://test.local/callback" {
		t.Errorf("Expected OAuth2RedirectURL to be 'https://test.local/callback', got '%s'", cfg.OAuth2RedirectURL)
	}
	if len(cfg.OAuth2Scopes) != 2 || cfg.OAuth2Scopes[0] != "openid" || cfg.OAuth2Scopes[1] != "profile" {
		t.Errorf("Expected OAuth2Scopes to be ['openid', 'profile'], got %v", cfg.OAuth2Scopes)
	}

	// Redis settings
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("Expected RedisAddr to be 'localhost:6379', got '%s'", cfg.RedisAddr)
	}
	if cfg.RedisPassword != "test-password" {
		t.Errorf("Expected RedisPassword to be 'test-password', got '%s'", cfg.RedisPassword)
	}
	if cfg.RedisDB != 1 {
		t.Errorf("Expected RedisDB to be 1, got %d", cfg.RedisDB)
	}
	if cfg.SessionTTL != 24*time.Hour {
		t.Errorf("Expected SessionTTL to be 24h, got %v", cfg.SessionTTL)
	}
	if cfg.SessionKeyPrefix != "test-session:" {
		t.Errorf("Expected SessionKeyPrefix to be 'test-session:', got '%s'", cfg.SessionKeyPrefix)
	}
	if cfg.StateKeyPrefix != "test-state:" {
		t.Errorf("Expected StateKeyPrefix to be 'test-state:', got '%s'", cfg.StateKeyPrefix)
	}

	// Session/Cookie settings
	if cfg.SessionCookieName != "TestSession" {
		t.Errorf("Expected SessionCookieName to be 'TestSession', got '%s'", cfg.SessionCookieName)
	}
	if cfg.SessionCookieSecure != false {
		t.Errorf("Expected SessionCookieSecure to be false, got %v", cfg.SessionCookieSecure)
	}

	// CORS settings
	if cfg.CORSAllowOrigin != "https://test.local" {
		t.Errorf("Expected CORSAllowOrigin to be 'https://test.local', got '%s'", cfg.CORSAllowOrigin)
	}
	if cfg.CORSAllowMethods != "GET, POST" {
		t.Errorf("Expected CORSAllowMethods to be 'GET, POST', got '%s'", cfg.CORSAllowMethods)
	}

	// TLS settings
	if cfg.RootCAFile != "/path/to/ca.crt" {
		t.Errorf("Expected RootCAFile to be '/path/to/ca.crt', got '%s'", cfg.RootCAFile)
	}
}

// TestGetConfigFromDotEnvFile tests loading configuration from .env file
func TestGetConfigFromDotEnvFile(t *testing.T) {
	resetSingleton()

	// 環境変数をクリアして .env ファイルからのみ読み込むようにする
	os.Clearenv()

	// テスト用の .env ファイルを作成
	envContent := `BFF_LISTEN_ADDR=:7070
PROXY_TARGET=http://envfile.test
OIDC_PROVIDER_URL=https://envfile.local/realms/test
OAUTH2_CLIENT_ID=envfile-client
OAUTH2_CLIENT_SECRET=envfile-secret
OAUTH2_REDIRECT_URL=https://envfile.local/callback
OAUTH2_SCOPES=openid,profile,email
REDIS_ADDR=envfile-redis:6379
REDIS_PASSWORD=envfile-password
REDIS_DB=2
SESSION_TTL=48h
SESSION_KEY_PREFIX=envfile-session:
STATE_KEY_PREFIX=envfile-state:
SESSION_COOKIE_NAME=EnvFileSession
SESSION_COOKIE_SECURE=false
CORS_ALLOW_ORIGIN=https://envfile.local
CORS_ALLOW_METHODS=GET, POST, PUT
ROOT_CA_FILE=/envfile/ca.crt`

	// 現在のディレクトリに .env ファイルを作成
	err := os.WriteFile(".env", []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create .env file: %v", err)
	}
	defer os.Remove(".env") // テスト終了後に削除

	cfg := GetConfig()

	// .env ファイルから読み込まれた値を検証
	if cfg.BFFListenAddr != ":7070" {
		t.Errorf("Expected BFFListenAddr to be ':7070', got '%s'", cfg.BFFListenAddr)
	}
	if cfg.ProxyTarget != "http://envfile.test" {
		t.Errorf("Expected ProxyTarget to be 'http://envfile.test', got '%s'", cfg.ProxyTarget)
	}
	if cfg.OAuth2ClientSecret != "envfile-secret" {
		t.Errorf("Expected OAuth2ClientSecret to be 'envfile-secret', got '%s'", cfg.OAuth2ClientSecret)
	}
	if cfg.RedisDB != 2 {
		t.Errorf("Expected RedisDB to be 2, got %d", cfg.RedisDB)
	}
	if cfg.SessionTTL != 48*time.Hour {
		t.Errorf("Expected SessionTTL to be 48h, got %v", cfg.SessionTTL)
	}
}

// TestGetConfigFromDotEnvTest tests loading configuration from .env.test file when GO_ENV=test
func TestGetConfigFromDotEnvTest(t *testing.T) {
	resetSingleton()

	// 環境変数をクリアして .env.test ファイルからのみ読み込むようにする
	os.Clearenv()
	os.Setenv("GO_ENV", "test")

	// テスト用の .env.test ファイルを作成
	envTestContent := `BFF_LISTEN_ADDR=:6060
PROXY_TARGET=http://testenv.test
OIDC_PROVIDER_URL=https://testenv.local/realms/test
OAUTH2_CLIENT_ID=testenv-client
OAUTH2_CLIENT_SECRET=testenv-secret
OAUTH2_REDIRECT_URL=https://testenv.local/callback
REDIS_ADDR=testenv-redis:6379
REDIS_DB=3
SESSION_TTL=12h
SESSION_COOKIE_SECURE=false`

	err := os.WriteFile(".env.test", []byte(envTestContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create .env.test file: %v", err)
	}
	defer os.Remove(".env.test")

	cfg := GetConfig()

	// .env.test ファイルから読み込まれた値を検証
	if cfg.BFFListenAddr != ":6060" {
		t.Errorf("Expected BFFListenAddr to be ':6060', got '%s'", cfg.BFFListenAddr)
	}
	if cfg.ProxyTarget != "http://testenv.test" {
		t.Errorf("Expected ProxyTarget to be 'http://testenv.test', got '%s'", cfg.ProxyTarget)
	}
	if cfg.OAuth2ClientSecret != "testenv-secret" {
		t.Errorf("Expected OAuth2ClientSecret to be 'testenv-secret', got '%s'", cfg.OAuth2ClientSecret)
	}
	if cfg.RedisDB != 3 {
		t.Errorf("Expected RedisDB to be 3, got %d", cfg.RedisDB)
	}
}

// TestGetConfigProductionIgnoresDotEnv tests that .env file is ignored when GO_ENV=production
func TestGetConfigProductionIgnoresDotEnv(t *testing.T) {
	resetSingleton()

	// 環境変数をクリアして GO_ENV=production を設定
	os.Clearenv()
	os.Setenv("GO_ENV", "production")

	// 本番環境では必須の環境変数を設定
	os.Setenv("BFF_LISTEN_ADDR", ":5050")
	os.Setenv("PROXY_TARGET", "http://production.test")
	os.Setenv("OAUTH2_CLIENT_SECRET", "production-secret")

	// .env ファイルを作成（本番環境では無視されるべき）
	envContent := `BFF_LISTEN_ADDR=:9999
PROXY_TARGET=http://should-be-ignored.test
OAUTH2_CLIENT_SECRET=should-be-ignored-secret`

	err := os.WriteFile(".env", []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create .env file: %v", err)
	}
	defer os.Remove(".env")

	cfg := GetConfig()

	// 環境変数の値が使われ、.env ファイルの値は無視されることを検証
	if cfg.BFFListenAddr != ":5050" {
		t.Errorf("Expected BFFListenAddr to be ':5050' (from env var, not .env file), got '%s'", cfg.BFFListenAddr)
	}
	if cfg.ProxyTarget != "http://production.test" {
		t.Errorf("Expected ProxyTarget to be 'http://production.test' (from env var, not .env file), got '%s'", cfg.ProxyTarget)
	}
	if cfg.OAuth2ClientSecret != "production-secret" {
		t.Errorf("Expected OAuth2ClientSecret to be 'production-secret' (from env var, not .env file), got '%s'", cfg.OAuth2ClientSecret)
	}

	// デフォルト値が使われることを確認（.env ファイルは無視されている）
	if cfg.RedisAddr != "redis:6379" {
		t.Errorf("Expected RedisAddr to be default 'redis:6379', got '%s'", cfg.RedisAddr)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "Valid regex pattern",
			cfg: &Config{
				AllowedRedirectPathPattern: "^/(dashboard|profile|settings)$",
			},
			wantErr: false,
		},
		{
			name: "Valid regex pattern with optional subpaths",
			cfg: &Config{
				AllowedRedirectPathPattern: "^/(dashboard|profile|settings)(/.*)?$",
			},
			wantErr: false,
		},
		{
			name: "Valid complex regex pattern",
			cfg: &Config{
				AllowedRedirectPathPattern: `^/(public|user/[^/]+/dashboard)$`,
			},
			wantErr: false,
		},
		{
			name: "Empty pattern (allowed for backward compatibility)",
			cfg: &Config{
				AllowedRedirectPathPattern: "",
			},
			wantErr: false,
		},
		{
			name: "Invalid regex pattern - unclosed bracket",
			cfg: &Config{
				AllowedRedirectPathPattern: "^/dashboard[",
			},
			wantErr: true,
		},
		{
			name: "Invalid regex pattern - unclosed parenthesis",
			cfg: &Config{
				AllowedRedirectPathPattern: "^/(dashboard|profile",
			},
			wantErr: true,
		},
		{
			name: "Invalid regex pattern - invalid escape sequence",
			cfg: &Config{
				AllowedRedirectPathPattern: `^\k`,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}