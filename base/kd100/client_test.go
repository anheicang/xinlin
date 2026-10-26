package kd100

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestQueryExpressUsesDocumentedRequestFormat(t *testing.T) {
	const customer = "customer"
	const key = "secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", req.Method)
		}
		if got := req.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q", got)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatal(err)
		}
		param := form.Get("param")
		wantSignBytes := md5.Sum([]byte(param + key + customer))
		wantSign := strings.ToUpper(fmt.Sprintf("%x", wantSignBytes))
		if form.Get("customer") != customer || form.Get("sign") != wantSign {
			t.Errorf("customer或sign不正确")
		}
		var query QueryRequest
		if err = json.Unmarshal([]byte(param), &query); err != nil {
			t.Fatal(err)
		}
		if query.Company != "ems" || query.Number != "em263999513jp" {
			t.Errorf("com/num = %q/%q", query.Company, query.Number)
		}
		if query.Phone != "13868688888" {
			t.Errorf("phone = %q, want full phone", query.Phone)
		}
		if query.ResultV2 != "4" {
			t.Errorf("resultv2 = %q, want 4", query.ResultV2)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"ok","status":"200","data":[]}`))
	}))
	defer server.Close()

	client := NewClient(customer, key, server.URL)
	response, err := client.QueryExpress(context.Background(), NewQueryRequest("EMS", "em263999513jp", "13868688888"))
	if err != nil || response == nil || !response.IsSuccess() {
		t.Fatalf("QueryExpress() response=%+v err=%v", response, err)
	}
}

func TestQueryExpressAllowsOptionalPhone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		form, _ := url.ParseQuery(string(body))
		if strings.Contains(form.Get("param"), `"phone"`) {
			t.Errorf("empty phone should be omitted: %s", form.Get("param"))
		}
		_, _ = w.Write([]byte(`{"message":"ok","status":"200","data":[]}`))
	}))
	defer server.Close()

	_, err := NewClient("customer", "secret", server.URL).QueryExpress(context.Background(), NewQueryRequest("ems", "123456", ""))
	if err != nil {
		t.Fatal(err)
	}
}

func TestQueryExpressRejectsInvalidTrackingNumberLength(t *testing.T) {
	_, err := NewClient("customer", "secret", "https://example.com").QueryExpress(context.Background(), NewQueryRequest("ems", "12345", ""))
	if err == nil || !strings.Contains(err.Error(), "6至32") {
		t.Fatalf("error = %v", err)
	}
}
