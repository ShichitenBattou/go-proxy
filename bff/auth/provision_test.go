package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProvisionUser_Success(t *testing.T) {
	var gotMethod, gotPath, gotAuth string

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
	}))
	defer backend.Close()

	err := provisionUser(context.Background(), "test-api-token", backend.Listener.Addr().String())

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/users" {
		t.Errorf("expected path /users, got %s", gotPath)
	}
	if gotAuth != "Bearer test-api-token" {
		t.Errorf("expected 'Bearer test-api-token', got %s", gotAuth)
	}
}

func TestProvisionUser_IdempotentOn201(t *testing.T) {
	callCount := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusCreated)
	}))
	defer backend.Close()

	for range 2 {
		if err := provisionUser(context.Background(), "token", backend.Listener.Addr().String()); err != nil {
			t.Fatalf("expected no error on call %d, got: %v", callCount, err)
		}
	}

	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestProvisionUser_APIError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer backend.Close()

	err := provisionUser(context.Background(), "token", backend.Listener.Addr().String())

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestProvisionUser_Non201Response(t *testing.T) {
	cases := []int{http.StatusOK, http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound}

	for _, statusCode := range cases {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
			}))
			defer backend.Close()

			err := provisionUser(context.Background(), "token", backend.Listener.Addr().String())

			if err == nil {
				t.Errorf("expected error for status %d, got nil", statusCode)
			}
		})
	}
}

func TestProvisionUser_ConnectionError(t *testing.T) {
	// 存在しないホストへ接続 → エラーになる
	err := provisionUser(context.Background(), "token", "127.0.0.1:19999")

	if err == nil {
		t.Fatal("expected error for unreachable host, got nil")
	}
}

func TestProvisionUser_ContextTimeout(t *testing.T) {
	// リクエストを受け取るが応答を返さないサーバー
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// コンテキストのキャンセルを待つ
		<-r.Context().Done()
	}))
	defer backend.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := provisionUser(ctx, "token", backend.Listener.Addr().String())

	if err == nil {
		t.Fatal("expected error for timed-out context, got nil")
	}
}
