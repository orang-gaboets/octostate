package client

import (
	"context"
	"testing"
	"time"
)

func TestNew_DefaultTimeout(t *testing.T) {
	ctx := context.Background()
	c := New(ctx, "token")
	if c.Client().Timeout != DefaultTimeout {
		t.Fatalf("expected timeout %v, got %v", DefaultTimeout, c.Client().Timeout)
	}
}

func TestNew_WithTimeout(t *testing.T) {
	ctx := context.Background()
	timeout := 10 * time.Second
	c := New(ctx, "token", WithTimeout(timeout))
	if c.Client().Timeout != timeout {
		t.Fatalf("expected timeout %v, got %v", timeout, c.Client().Timeout)
	}
}
