package api

import "testing"

func TestNewRequest(t *testing.T) {
	req, err := Client{BaseURL: "https://partner.example", Token: "tok"}.NewRequest("/orders")
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.String() != "https://partner.example/orders" || req.Header.Get("Authorization") != "Bearer tok" {
		t.Fatalf("req = %v %v", req.URL, req.Header)
	}
}
