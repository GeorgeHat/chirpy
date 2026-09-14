package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeJWT(t *testing.T) {
	user_id := uuid.New()
	secret_key := "secret123"
	expiresIn, err := time.ParseDuration("5s")
	if err != nil {
		t.Errorf("Invalid duration format")
	}
	_, err = MakeJWT(user_id, secret_key, expiresIn)
	if err != nil {
		t.Errorf("Error Creating JWT")
	}
}

func TestValidateJWTCorrect(t *testing.T) {
	user_id := uuid.New()
	secret_key := "secret123"
	expiresIn, _ := time.ParseDuration("5s")
	signature, _ := MakeJWT(user_id, secret_key, expiresIn)
	test_id, err := ValidateJWT(signature, secret_key)
	if err != nil {
		t.Error(err)
		return
	}
	if test_id != user_id {
		t.Errorf("Expected id: %v, Output id %v", user_id, test_id)
	}

}

func TestValidateJWTEmpty(t *testing.T) {
	user_id := uuid.New()
	secret_key := ""
	expiresIn, _ := time.ParseDuration("5s")
	signature, _ := MakeJWT(user_id, secret_key, expiresIn)
	test_id, err := ValidateJWT(signature, secret_key)
	if err != nil {
		t.Error(err)
		return
	}
	if test_id != user_id {
		t.Errorf("Expected id: %v, Output id %v", user_id, test_id)
	}

}

func TestValidateJWTWrongKey(t *testing.T) {
	user_id := uuid.New()
	secret_key := "secret123"
	expiresIn, _ := time.ParseDuration("5s")
	signature, _ := MakeJWT(user_id, secret_key, expiresIn)
	_, err := ValidateJWT(signature, "wrong_key")
	if err == nil {
		t.Error("Different key should not work")
		return
	}

}

func TestValidateJWTExpired(t *testing.T) {
	user_id := uuid.New()
	secret_key := "secret123"
	expiresIn, _ := time.ParseDuration("5s")
	signature, _ := MakeJWT(user_id, secret_key, expiresIn)
	wait, _ := time.ParseDuration("6s")
	time.Sleep(wait)
	_, err := ValidateJWT(signature, secret_key)
	if err == nil {
		t.Error("Expired Key should not work")
		return
	}

}
