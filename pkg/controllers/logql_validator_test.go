package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateLogQLOnServerFunc(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		if r.URL.Path != "/loki/api/v1/query" {
			t.Errorf("The request URL should be /loki/api/v1/query")
		}

		logQLQuery := r.URL.Query().Get("query")
		if logQLQuery != "{job=\"loki-test\"}" {
			t.Errorf("The logQL query should be {job=\"loki-test\"}")
		}
	}))

	defer ts.Close()

	isValid, err := ValidateLogQLOnServerFunc(ts.URL, "{job=\"loki-test\"}")

	if err != nil {
		t.Errorf("Error: %v", err)
	}

	if isValid == false {
		t.Errorf("The server should return HTTP 500: %v", err)
	}
}

func TestValidateLogQLOnServerFuncHTTP500IsAnInvalidResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	defer ts.Close()

	isValid, err := ValidateLogQLOnServerFunc(ts.URL, "{job=\"loki-test\"}")

	if err != nil {
		t.Errorf("Error: %v", err)
	}

	if isValid == true {
		t.Errorf("The logQL is invalid")
	}
}

func TestValidateLogQLOnServerFuncInvalidRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	isValid, err := ValidateLogQLOnServerFunc(ts.URL, "{job=\"loki-test\"}")

	if err != nil {
		t.Errorf("Error: %v", err)
	}

	if isValid == true {
		t.Errorf("The logQL is invalid")
	}
}
